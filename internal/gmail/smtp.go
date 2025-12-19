package gmail

import (
	"fmt"
	"net/smtp"

	"maily/internal/auth"
)

type SMTPClient struct {
	creds *auth.Credentials
}

func NewSMTPClient(creds *auth.Credentials) *SMTPClient {
	return &SMTPClient{creds: creds}
}

func (c *SMTPClient) Send(to, subject, body string) error {
	addr := fmt.Sprintf("%s:%d", c.creds.SMTPHost, c.creds.SMTPPort)

	auth := smtp.PlainAuth("", c.creds.Email, c.creds.Password, c.creds.SMTPHost)

	msg := fmt.Sprintf("From: %s\r\n"+
		"To: %s\r\n"+
		"Subject: %s\r\n"+
		"MIME-Version: 1.0\r\n"+
		"Content-Type: text/plain; charset=\"utf-8\"\r\n"+
		"\r\n"+
		"%s", c.creds.Email, to, subject, body)

	return smtp.SendMail(addr, auth, c.creds.Email, []string{to}, []byte(msg))
}

func (c *SMTPClient) Reply(to, subject, body, inReplyTo, references string) error {
	addr := fmt.Sprintf("%s:%d", c.creds.SMTPHost, c.creds.SMTPPort)

	auth := smtp.PlainAuth("", c.creds.Email, c.creds.Password, c.creds.SMTPHost)

	if references == "" {
		references = inReplyTo
	} else {
		references = references + " " + inReplyTo
	}

	msg := fmt.Sprintf("From: %s\r\n"+
		"To: %s\r\n"+
		"Subject: %s\r\n"+
		"In-Reply-To: %s\r\n"+
		"References: %s\r\n"+
		"MIME-Version: 1.0\r\n"+
		"Content-Type: text/plain; charset=\"utf-8\"\r\n"+
		"\r\n"+
		"%s", c.creds.Email, to, subject, inReplyTo, references, body)

	return smtp.SendMail(addr, auth, c.creds.Email, []string{to}, []byte(msg))
}
