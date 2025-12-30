package mail

import (
	"fmt"
	"net/smtp"
	"strings"

	"maily/internal/auth"
)

// sanitizeHeader removes CRLF sequences to prevent header injection attacks
func sanitizeHeader(s string) string {
	s = strings.ReplaceAll(s, "\r\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	return s
}

type SMTPClient struct {
	creds *auth.Credentials
}

func NewSMTPClient(creds *auth.Credentials) *SMTPClient {
	return &SMTPClient{creds: creds}
}

func (c *SMTPClient) Send(to, subject, body string) error {
	addr := fmt.Sprintf("%s:%d", c.creds.SMTPHost, c.creds.SMTPPort)

	auth := smtp.PlainAuth("", c.creds.Email, c.creds.Password, c.creds.SMTPHost)

	// Sanitize headers to prevent CRLF injection
	to = sanitizeHeader(to)
	subject = sanitizeHeader(subject)

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

	// Sanitize headers to prevent CRLF injection
	to = sanitizeHeader(to)
	subject = sanitizeHeader(subject)
	inReplyTo = sanitizeHeader(inReplyTo)
	references = sanitizeHeader(references)

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
