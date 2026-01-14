use rusqlite::params;
use serde::{Deserialize, Serialize};
use std::process::Command;

use crate::config::{get_config, AIProviderType};
use crate::mail::DB;

/// Email summary stored in database
#[derive(Debug, Serialize, Deserialize, Clone)]
pub struct EmailSummary {
    pub email_uid: u32,
    pub account: String,
    pub mailbox: String,
    pub summary: String,
    pub model_used: String,
    pub created_at: i64,
}

/// AI completion request
#[derive(Debug, Serialize, Deserialize)]
pub struct CompletionRequest {
    pub prompt: String,
    pub system_prompt: Option<String>,
    pub max_tokens: Option<u32>,
    /// Optional provider name to use (if not specified, uses first available)
    pub provider_name: Option<String>,
}

/// AI completion response
#[derive(Debug, Serialize, Deserialize, Clone)]
pub struct CompletionResponse {
    pub success: bool,
    pub content: Option<String>,
    pub error: Option<String>,
    pub model_used: Option<String>,
}

// Initialize summaries table
pub fn init_summaries_table() {
    let conn = DB.lock().unwrap();
    conn.execute_batch(r#"
        CREATE TABLE IF NOT EXISTS email_summaries (
            account TEXT NOT NULL,
            mailbox TEXT NOT NULL,
            email_uid INTEGER NOT NULL,
            summary TEXT NOT NULL,
            model_used TEXT NOT NULL,
            created_at INTEGER NOT NULL,
            PRIMARY KEY (account, mailbox, email_uid)
        );
    "#).expect("Failed to create summaries table");
}

/// Get cached summary for an email
pub fn get_cached_summary(account: &str, mailbox: &str, uid: u32) -> Option<EmailSummary> {
    let conn = DB.lock().unwrap();
    let mut stmt = conn.prepare(
        "SELECT summary, model_used, created_at FROM email_summaries WHERE account = ?1 AND mailbox = ?2 AND email_uid = ?3"
    ).ok()?;

    stmt.query_row(params![account, mailbox, uid], |row| {
        Ok(EmailSummary {
            email_uid: uid,
            account: account.to_string(),
            mailbox: mailbox.to_string(),
            summary: row.get(0)?,
            model_used: row.get(1)?,
            created_at: row.get(2)?,
        })
    }).ok()
}

/// Save summary to cache
pub fn save_summary(summary: &EmailSummary) -> Result<(), Box<dyn std::error::Error>> {
    let conn = DB.lock().unwrap();
    conn.execute(
        "INSERT OR REPLACE INTO email_summaries (account, mailbox, email_uid, summary, model_used, created_at) VALUES (?1, ?2, ?3, ?4, ?5, ?6)",
        params![
            summary.account,
            summary.mailbox,
            summary.email_uid,
            summary.summary,
            summary.model_used,
            summary.created_at
        ]
    )?;
    Ok(())
}

/// Delete cached summary
pub fn delete_summary(account: &str, mailbox: &str, uid: u32) -> Result<(), Box<dyn std::error::Error>> {
    let conn = DB.lock().unwrap();
    conn.execute(
        "DELETE FROM email_summaries WHERE account = ?1 AND mailbox = ?2 AND email_uid = ?3",
        params![account, mailbox, uid]
    )?;
    Ok(())
}

/// Call AI provider for completion
pub fn complete(request: CompletionRequest) -> CompletionResponse {
    let config = match get_config() {
        Ok(c) => c,
        Err(e) => return CompletionResponse {
            success: false,
            content: None,
            error: Some(format!("Failed to load config: {}", e)),
            model_used: None,
        },
    };

    // If a specific provider is requested, try it first
    if let Some(ref provider_name) = request.provider_name {
        // Check configured providers
        if let Some(provider) = config.ai_providers.iter().find(|p| &p.name == provider_name) {
            let result = match provider.provider_type {
                AIProviderType::Cli => call_cli_provider(&provider.name, &provider.model, &request),
                AIProviderType::Api => call_api_provider(&provider.name, &provider.model, &provider.base_url, &provider.api_key, &request),
            };
            if result.success {
                return CompletionResponse {
                    model_used: Some(format!("{}/{}", provider.name, provider.model)),
                    ..result
                };
            }
        }

        // Check if it's a CLI tool
        let cli_tools = vec![
            ("claude", "haiku"),
            ("codex", "o4-mini"),
            ("gemini", "flash"),
            ("ollama", "llama3.2"),
        ];

        if let Some((_, model)) = cli_tools.iter().find(|(name, _)| name == provider_name) {
            if is_cli_available(provider_name) {
                let result = call_cli_provider(provider_name, model, &request);
                if result.success {
                    return CompletionResponse {
                        model_used: Some(format!("{}/{}", provider_name, model)),
                        ..result
                    };
                }
            }
        }
    }

    // Try configured providers in order
    for provider in &config.ai_providers {
        let result = match provider.provider_type {
            AIProviderType::Cli => call_cli_provider(&provider.name, &provider.model, &request),
            AIProviderType::Api => call_api_provider(&provider.name, &provider.model, &provider.base_url, &provider.api_key, &request),
        };

        if result.success {
            return CompletionResponse {
                model_used: Some(format!("{}/{}", provider.name, provider.model)),
                ..result
            };
        }
    }

    // Try auto-detecting CLI tools
    let cli_tools = vec![
        ("claude", "haiku"),
        ("codex", "o4-mini"),
        ("gemini", "flash"),
        ("ollama", "llama3.2"),
    ];

    for (name, model) in cli_tools {
        if is_cli_available(name) {
            let result = call_cli_provider(name, model, &request);
            if result.success {
                return CompletionResponse {
                    model_used: Some(format!("{}/{}", name, model)),
                    ..result
                };
            }
        }
    }

    CompletionResponse {
        success: false,
        content: None,
        error: Some("No AI provider available. Please configure one in Settings.".to_string()),
        model_used: None,
    }
}

/// Test a specific AI provider
pub fn test_provider(provider_name: &str, provider_model: &str, provider_type: &str, base_url: &str, api_key: &str) -> CompletionResponse {
    let request = CompletionRequest {
        prompt: "Say 'Hello! I am working.' and nothing else.".to_string(),
        system_prompt: Some("You are a test assistant. Follow instructions exactly.".to_string()),
        max_tokens: Some(20),
        provider_name: None,
    };

    if provider_type == "cli" {
        call_cli_provider(provider_name, provider_model, &request)
    } else {
        call_api_provider(provider_name, provider_model, base_url, api_key, &request)
    }
}

/// List all available AI providers
pub fn list_available_providers() -> Vec<String> {
    let mut providers = Vec::new();

    // Add configured providers
    if let Ok(config) = get_config() {
        for provider in &config.ai_providers {
            providers.push(provider.name.clone());
        }
    }

    // Add available CLI tools
    let cli_tools = ["claude", "codex", "gemini", "ollama"];
    for tool in cli_tools {
        if is_cli_available(tool) && !providers.contains(&tool.to_string()) {
            providers.push(tool.to_string());
        }
    }

    providers
}

fn is_cli_available(name: &str) -> bool {
    Command::new("which")
        .arg(name)
        .output()
        .map(|o| o.status.success())
        .unwrap_or(false)
}

fn call_cli_provider(name: &str, model: &str, request: &CompletionRequest) -> CompletionResponse {
    let prompt = if let Some(ref sys) = request.system_prompt {
        format!("{}\n\n{}", sys, request.prompt)
    } else {
        request.prompt.clone()
    };

    let result = match name {
        "claude" => {
            Command::new("claude")
                .args(["--model", model, "-p", &prompt])
                .output()
        }
        "codex" => {
            Command::new("codex")
                .args(["-m", model, "-p", &prompt])
                .output()
        }
        "gemini" => {
            Command::new("gemini")
                .args(["-m", model, "-p", &prompt])
                .output()
        }
        "ollama" => {
            Command::new("ollama")
                .args(["run", model, &prompt])
                .output()
        }
        _ => {
            // Generic CLI that accepts -p for prompt
            Command::new(name)
                .args(["-m", model, "-p", &prompt])
                .output()
        }
    };

    match result {
        Ok(output) => {
            if output.status.success() {
                let content = String::from_utf8_lossy(&output.stdout).trim().to_string();
                CompletionResponse {
                    success: true,
                    content: Some(content),
                    error: None,
                    model_used: Some(format!("{}/{}", name, model)),
                }
            } else {
                let stderr = String::from_utf8_lossy(&output.stderr).to_string();
                CompletionResponse {
                    success: false,
                    content: None,
                    error: Some(stderr),
                    model_used: None,
                }
            }
        }
        Err(e) => CompletionResponse {
            success: false,
            content: None,
            error: Some(e.to_string()),
            model_used: None,
        },
    }
}

fn call_api_provider(name: &str, model: &str, base_url: &str, api_key: &str, request: &CompletionRequest) -> CompletionResponse {
    // Use blocking reqwest for simplicity (called from sync context)
    let client = match reqwest::blocking::Client::builder()
        .timeout(std::time::Duration::from_secs(60))
        .build() {
        Ok(c) => c,
        Err(e) => return CompletionResponse {
            success: false,
            content: None,
            error: Some(e.to_string()),
            model_used: None,
        },
    };

    // Build OpenAI-compatible request
    let mut messages = vec![];
    if let Some(ref sys) = request.system_prompt {
        messages.push(serde_json::json!({
            "role": "system",
            "content": sys
        }));
    }
    messages.push(serde_json::json!({
        "role": "user",
        "content": request.prompt
    }));

    let body = serde_json::json!({
        "model": model,
        "messages": messages,
        "max_tokens": request.max_tokens.unwrap_or(1000),
    });

    let url = format!("{}/chat/completions", base_url.trim_end_matches('/'));

    let response = client
        .post(&url)
        .header("Authorization", format!("Bearer {}", api_key))
        .header("Content-Type", "application/json")
        .json(&body)
        .send();

    match response {
        Ok(resp) => {
            if resp.status().is_success() {
                match resp.json::<serde_json::Value>() {
                    Ok(json) => {
                        let content = json["choices"][0]["message"]["content"]
                            .as_str()
                            .map(|s| s.to_string());

                        CompletionResponse {
                            success: content.is_some(),
                            content,
                            error: None,
                            model_used: Some(format!("{}/{}", name, model)),
                        }
                    }
                    Err(e) => CompletionResponse {
                        success: false,
                        content: None,
                        error: Some(format!("Failed to parse response: {}", e)),
                        model_used: None,
                    },
                }
            } else {
                let status = resp.status();
                let body = resp.text().unwrap_or_default();
                CompletionResponse {
                    success: false,
                    content: None,
                    error: Some(format!("API error {}: {}", status, body)),
                    model_used: None,
                }
            }
        }
        Err(e) => CompletionResponse {
            success: false,
            content: None,
            error: Some(e.to_string()),
            model_used: None,
        },
    }
}

/// Summarize email content
pub fn summarize_email(
    account: &str,
    mailbox: &str,
    uid: u32,
    subject: &str,
    from: &str,
    body_text: &str,
    force_refresh: bool,
) -> CompletionResponse {
    // Check cache first (unless force refresh)
    if !force_refresh {
        if let Some(cached) = get_cached_summary(account, mailbox, uid) {
            return CompletionResponse {
                success: true,
                content: Some(cached.summary),
                error: None,
                model_used: Some(cached.model_used),
            };
        }
    }

    // Truncate body if too long
    let body_truncated: String = body_text.chars().take(4000).collect();

    let prompt = format!(
        r#"Summarize this email as bullet points.

From: {}
Subject: {}

{}

Format your response exactly like this (skip sections if not applicable):

Summary:
    <one sentence summary>

Key Points:
    - <point 1>
    - <point 2>

Action Items:
    - <action 1>
    - <action 2>

Dates/Deadlines:
    - <date/deadline if mentioned>

Keep it brief. No preamble, section titles on their own line, content indented with 4 spaces."#,
        from, subject, body_truncated
    );

    let response = complete(CompletionRequest {
        prompt,
        system_prompt: None,
        max_tokens: Some(500),
        provider_name: None,
    });

    // Cache successful response
    if response.success {
        if let Some(ref content) = response.content {
            let now = std::time::SystemTime::now()
                .duration_since(std::time::UNIX_EPOCH)
                .unwrap_or_default()
                .as_secs() as i64;

            let summary = EmailSummary {
                email_uid: uid,
                account: account.to_string(),
                mailbox: mailbox.to_string(),
                summary: content.clone(),
                model_used: response.model_used.clone().unwrap_or_default(),
                created_at: now,
            };

            let _ = save_summary(&summary);
        }
    }

    response
}

/// Generate smart reply suggestions
pub fn generate_reply(
    original_from: &str,
    original_subject: &str,
    original_body: &str,
    reply_intent: &str, // e.g., "accept", "decline", "ask for more info"
) -> CompletionResponse {
    let body_truncated: String = original_body.chars().take(2000).collect();

    let prompt = format!(
        r#"Generate a professional email reply. The intent is to: {}

Original email:
From: {}
Subject: {}
Body: {}

Write a concise, professional reply:"#,
        reply_intent, original_from, original_subject, body_truncated
    );

    complete(CompletionRequest {
        prompt,
        system_prompt: Some("You are a professional email writer. Write concise, clear, and polite emails.".to_string()),
        max_tokens: Some(500),
        provider_name: None,
    })
}

/// Extract calendar event from email
pub fn extract_event(
    subject: &str,
    body_text: &str,
) -> CompletionResponse {
    let body_truncated: String = body_text.chars().take(3000).collect();

    // Get current time in RFC3339 format
    let now = chrono::Local::now().format("%Y-%m-%dT%H:%M:%S%:z").to_string();

    let prompt = format!(
        r#"Extract the most relevant calendar event, meeting, or deadline from this email.

Current date/time: {}

Subject: {}

{}

If an event is found, respond with ONLY a JSON object (no markdown, no explanation):
{{
  "title": "event title",
  "start_time": "2024-12-25T10:00:00-08:00",
  "end_time": "2024-12-25T11:00:00-08:00",
  "location": "location if mentioned, otherwise empty string",
  "alarm_minutes_before": 0,
  "alarm_specified": false
}}

If NO events found, respond with exactly: NO_EVENTS_FOUND

Rules:
- start_time and end_time must be in RFC3339 format with timezone
- If no end time/duration specified, default to 1 hour after start
- Extract location if mentioned
- Use the current date/time to interpret relative dates like "tomorrow", "next Monday"
- Pick the most important/relevant event if multiple are mentioned
- Set alarm_minutes_before=0 and alarm_specified=false (user will set reminder later)

Respond with ONLY the JSON or NO_EVENTS_FOUND, no other text."#,
        now, subject, body_truncated
    );

    complete(CompletionRequest {
        prompt,
        system_prompt: None,
        max_tokens: Some(300),
        provider_name: None,
    })
}
