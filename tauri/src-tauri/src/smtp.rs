use lettre::{
    message::{header::ContentType, Attachment, Mailbox, MultiPart, SinglePart},
    transport::smtp::authentication::Credentials,
    Message, SmtpTransport, Transport,
};
use serde::{Deserialize, Serialize};
use std::fs;
use std::path::PathBuf;

use crate::mail::{get_accounts, Account};

#[derive(Debug, Serialize, Deserialize, Clone)]
pub struct ComposeEmail {
    pub to: Vec<String>,
    pub cc: Vec<String>,
    pub bcc: Vec<String>,
    pub subject: String,
    pub body_html: String,
    pub body_text: String,
    pub attachments: Vec<AttachmentInfo>,
    pub reply_to_message_id: Option<String>,
    pub references: Option<String>,
}

#[derive(Debug, Serialize, Deserialize, Clone)]
pub struct AttachmentInfo {
    pub path: String,
    pub filename: String,
    pub content_type: String,
}

#[derive(Debug, Serialize, Deserialize, Clone)]
pub struct SendResult {
    pub success: bool,
    pub message_id: Option<String>,
    pub error: Option<String>,
}

fn get_account(name: &str) -> Result<Account, Box<dyn std::error::Error>> {
    let accounts = get_accounts()?;
    accounts
        .into_iter()
        .find(|a| a.name == name)
        .ok_or_else(|| format!("Account '{}' not found", name).into())
}

pub fn send_email(account_name: &str, email: ComposeEmail) -> Result<SendResult, Box<dyn std::error::Error>> {
    let account = get_account(account_name)?;
    let creds = &account.credentials;

    // Parse the from address
    let from_mailbox: Mailbox = creds.email.parse()
        .map_err(|_| format!("Invalid from email: {}", creds.email))?;

    // Build the message
    let mut builder = Message::builder()
        .from(from_mailbox)
        .subject(&email.subject);

    // Add To recipients
    for to in &email.to {
        let mailbox: Mailbox = to.parse()
            .map_err(|_| format!("Invalid to email: {}", to))?;
        builder = builder.to(mailbox);
    }

    // Add Cc recipients
    for cc in &email.cc {
        let mailbox: Mailbox = cc.parse()
            .map_err(|_| format!("Invalid cc email: {}", cc))?;
        builder = builder.cc(mailbox);
    }

    // Add Bcc recipients
    for bcc in &email.bcc {
        let mailbox: Mailbox = bcc.parse()
            .map_err(|_| format!("Invalid bcc email: {}", bcc))?;
        builder = builder.bcc(mailbox);
    }

    // Add threading headers for replies
    if let Some(ref reply_to) = email.reply_to_message_id {
        builder = builder.in_reply_to(reply_to.clone());
    }
    if let Some(ref refs) = email.references {
        builder = builder.references(refs.clone());
    }

    // Build the body - lettre MultiPart doesn't need .build()
    let message = if email.attachments.is_empty() {
        // Simple message without attachments
        if !email.body_html.is_empty() {
            // Multipart alternative with text and HTML
            let multipart = MultiPart::alternative()
                .singlepart(
                    SinglePart::builder()
                        .header(ContentType::TEXT_PLAIN)
                        .body(email.body_text.clone())
                )
                .singlepart(
                    SinglePart::builder()
                        .header(ContentType::TEXT_HTML)
                        .body(email.body_html.clone())
                );
            builder.multipart(multipart)?
        } else {
            // Plain text only
            builder.body(email.body_text.clone())?
        }
    } else {
        // Message with attachments - build all parts first
        let body_part = if !email.body_html.is_empty() {
            MultiPart::alternative()
                .singlepart(
                    SinglePart::builder()
                        .header(ContentType::TEXT_PLAIN)
                        .body(email.body_text.clone())
                )
                .singlepart(
                    SinglePart::builder()
                        .header(ContentType::TEXT_HTML)
                        .body(email.body_html.clone())
                )
        } else {
            MultiPart::mixed()
                .singlepart(
                    SinglePart::builder()
                        .header(ContentType::TEXT_PLAIN)
                        .body(email.body_text.clone())
                )
        };

        let mut mixed = MultiPart::mixed().multipart(body_part);

        // Add attachments
        for att_info in &email.attachments {
            let path = PathBuf::from(&att_info.path);
            let content = fs::read(&path)?;
            let content_type: ContentType = att_info.content_type.parse()
                .unwrap_or(ContentType::parse("application/octet-stream").unwrap());

            let attachment = Attachment::new(att_info.filename.clone())
                .body(content, content_type);

            mixed = mixed.singlepart(attachment);
        }

        builder.multipart(mixed)?
    };

    // Create SMTP transport
    let smtp_creds = Credentials::new(creds.email.clone(), creds.password.clone());

    let mailer = SmtpTransport::starttls_relay(&creds.smtp_host)?
        .port(creds.smtp_port)
        .credentials(smtp_creds)
        .build();

    // Send the message
    match mailer.send(&message) {
        Ok(response) => {
            eprintln!("[smtp] Email sent successfully: {:?}", response);
            Ok(SendResult {
                success: true,
                message_id: None, // Message-ID extraction requires different approach
                error: None,
            })
        }
        Err(e) => {
            eprintln!("[smtp] Failed to send email: {}", e);
            Ok(SendResult {
                success: false,
                message_id: None,
                error: Some(e.to_string()),
            })
        }
    }
}

/// Save email to drafts folder via IMAP APPEND
pub fn save_draft_to_imap(account_name: &str, email: &ComposeEmail) -> Result<(), Box<dyn std::error::Error>> {
    use std::net::{TcpStream, ToSocketAddrs};
    use std::time::Duration;

    let account = get_account(account_name)?;
    let creds = &account.credentials;

    // Build RFC 822 message using lettre
    let from_mailbox: Mailbox = creds.email.parse()
        .map_err(|_| format!("Invalid from email: {}", creds.email))?;

    let mut builder = Message::builder()
        .from(from_mailbox)
        .subject(&email.subject);

    // Add To recipients
    for to in &email.to {
        if !to.is_empty() {
            let mailbox: Mailbox = to.parse()
                .map_err(|_| format!("Invalid to email: {}", to))?;
            builder = builder.to(mailbox);
        }
    }

    // Add Cc recipients
    for cc in &email.cc {
        if !cc.is_empty() {
            let mailbox: Mailbox = cc.parse()
                .map_err(|_| format!("Invalid cc email: {}", cc))?;
            builder = builder.cc(mailbox);
        }
    }

    // Build the body
    let message = if !email.body_html.is_empty() {
        let multipart = MultiPart::alternative()
            .singlepart(
                SinglePart::builder()
                    .header(ContentType::TEXT_PLAIN)
                    .body(email.body_text.clone())
            )
            .singlepart(
                SinglePart::builder()
                    .header(ContentType::TEXT_HTML)
                    .body(email.body_html.clone())
            );
        builder.multipart(multipart)?
    } else {
        builder.body(email.body_text.clone())?
    };

    // Convert to RFC 822 bytes
    let rfc822_bytes = message.formatted();

    // Connect to IMAP
    let addr = format!("{}:{}", creds.imap_host, creds.imap_port);
    let socket_addr = addr.to_socket_addrs()?.next()
        .ok_or("Failed to resolve IMAP host")?;

    let tcp = TcpStream::connect_timeout(&socket_addr, Duration::from_secs(30))?;
    tcp.set_read_timeout(Some(Duration::from_secs(60)))?;
    tcp.set_write_timeout(Some(Duration::from_secs(30)))?;

    let tls = native_tls::TlsConnector::builder().build()?;
    let tls_stream = tls.connect(&creds.imap_host, tcp)?;

    let client = imap::Client::new(tls_stream);
    let mut session = client.login(&creds.email, &creds.password).map_err(|e| e.0)?;

    // Determine drafts folder based on provider
    let drafts_folder = if creds.imap_host.contains("gmail") {
        "[Gmail]/Drafts"
    } else if creds.imap_host.contains("yahoo") {
        "Draft"
    } else {
        "Drafts"
    };

    // Append to drafts folder with \Draft flag
    session.append_with_flags(drafts_folder, &rfc822_bytes, &[imap::types::Flag::Draft])?;

    session.logout()?;
    eprintln!("[smtp] Draft saved to IMAP: {}", email.subject);
    Ok(())
}
