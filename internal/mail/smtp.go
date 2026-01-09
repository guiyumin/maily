package mail

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"mime/quotedprintable"
	"net/smtp"
	"os"
	"path/filepath"
	"strings"

	"maily/internal/auth"
)

// AttachmentFile represents an email attachment
type AttachmentFile struct {
	Path        string
	Name        string
	Size        int64
	ContentType string
}

// sanitizeHeader removes CRLF sequences to prevent header injection attacks
func sanitizeHeader(s string) string {
	s = strings.ReplaceAll(s, "\r\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	return s
}

// sanitizeContentType validates and sanitizes a MIME content type to prevent header injection.
// It parses the content type and rebuilds it safely, falling back to application/octet-stream on error.
func sanitizeContentType(contentType string) string {
	if contentType == "" {
		return "application/octet-stream"
	}

	// Parse the media type to validate it
	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		return "application/octet-stream"
	}

	// Check for control characters in the media type
	for _, r := range mediaType {
		if r == '\r' || r == '\n' || (r < 32 && r != '\t') {
			return "application/octet-stream"
		}
	}

	// Rebuild safely using FormatMediaType (which properly escapes parameters)
	return mime.FormatMediaType(mediaType, params)
}

// encodeFilename safely encodes a filename for use in MIME headers.
// It strips control characters, escapes quotes, and uses MIME Q-encoding for non-ASCII.
func encodeFilename(name string) string {
	// Strip CR, LF, and other control characters
	var sb strings.Builder
	hasNonASCII := false
	for _, r := range name {
		if r == '\r' || r == '\n' || (r < 32 && r != '\t') {
			continue
		}
		if r > 127 {
			hasNonASCII = true
		}
		sb.WriteRune(r)
	}
	clean := sb.String()

	// If non-ASCII, use MIME Q-encoding (RFC 2047)
	if hasNonASCII {
		return mime.QEncoding.Encode("utf-8", clean)
	}

	// For ASCII, escape quotes and backslashes
	clean = strings.ReplaceAll(clean, "\\", "\\\\")
	clean = strings.ReplaceAll(clean, "\"", "\\\"")
	return clean
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

// SendWithAttachments sends an email with attachments
func (c *SMTPClient) SendWithAttachments(to, subject, body string, attachments []AttachmentFile) error {
	if len(attachments) == 0 {
		return c.Send(to, subject, body)
	}

	addr := fmt.Sprintf("%s:%d", c.creds.SMTPHost, c.creds.SMTPPort)
	auth := smtp.PlainAuth("", c.creds.Email, c.creds.Password, c.creds.SMTPHost)

	// Sanitize headers
	to = sanitizeHeader(to)
	subject = sanitizeHeader(subject)

	msg, err := buildMultipartMessage(c.creds.Email, to, subject, body, "", "", attachments)
	if err != nil {
		return fmt.Errorf("failed to build message: %w", err)
	}

	return smtp.SendMail(addr, auth, c.creds.Email, []string{to}, msg)
}

// ReplyWithAttachments sends a reply email with attachments
func (c *SMTPClient) ReplyWithAttachments(to, subject, body, inReplyTo, references string, attachments []AttachmentFile) error {
	if len(attachments) == 0 {
		return c.Reply(to, subject, body, inReplyTo, references)
	}

	addr := fmt.Sprintf("%s:%d", c.creds.SMTPHost, c.creds.SMTPPort)
	auth := smtp.PlainAuth("", c.creds.Email, c.creds.Password, c.creds.SMTPHost)

	// Sanitize headers
	to = sanitizeHeader(to)
	subject = sanitizeHeader(subject)
	inReplyTo = sanitizeHeader(inReplyTo)
	references = sanitizeHeader(references)

	if references == "" {
		references = inReplyTo
	} else {
		references = references + " " + inReplyTo
	}

	msg, err := buildMultipartMessage(c.creds.Email, to, subject, body, inReplyTo, references, attachments)
	if err != nil {
		return fmt.Errorf("failed to build message: %w", err)
	}

	return smtp.SendMail(addr, auth, c.creds.Email, []string{to}, msg)
}

// buildMultipartMessage constructs a MIME multipart message with attachments
func buildMultipartMessage(from, to, subject, body, inReplyTo, references string, attachments []AttachmentFile) ([]byte, error) {
	var buf bytes.Buffer

	// Generate a unique boundary
	boundary := fmt.Sprintf("----=_Part_%s", randomBoundary())

	// Write headers
	buf.WriteString(fmt.Sprintf("From: %s\r\n", from))
	buf.WriteString(fmt.Sprintf("To: %s\r\n", to))
	buf.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))

	// Add reply headers if this is a reply
	if inReplyTo != "" {
		buf.WriteString(fmt.Sprintf("In-Reply-To: %s\r\n", inReplyTo))
	}
	if references != "" {
		buf.WriteString(fmt.Sprintf("References: %s\r\n", references))
	}

	buf.WriteString("MIME-Version: 1.0\r\n")
	buf.WriteString(fmt.Sprintf("Content-Type: multipart/mixed; boundary=\"%s\"\r\n", boundary))
	buf.WriteString("\r\n")

	// Write text body part with quoted-printable encoding for UTF-8 safety
	buf.WriteString(fmt.Sprintf("--%s\r\n", boundary))
	buf.WriteString("Content-Type: text/plain; charset=\"utf-8\"\r\n")
	buf.WriteString("Content-Transfer-Encoding: quoted-printable\r\n")
	buf.WriteString("\r\n")
	qpWriter := quotedprintable.NewWriter(&buf)
	qpWriter.Write([]byte(body))
	qpWriter.Close()
	buf.WriteString("\r\n")

	// Write attachment parts
	for _, att := range attachments {
		// Open file for streaming instead of reading all into memory
		file, err := os.Open(att.Path)
		if err != nil {
			return nil, fmt.Errorf("failed to open attachment %s: %w", att.Name, err)
		}

		// Determine and sanitize content type to prevent header injection
		contentType := att.ContentType
		if contentType == "" {
			contentType = mime.TypeByExtension(filepath.Ext(att.Name))
		}
		contentType = sanitizeContentType(contentType)

		// Safely encode filename for MIME headers
		encodedName := encodeFilename(att.Name)

		buf.WriteString(fmt.Sprintf("--%s\r\n", boundary))
		buf.WriteString(fmt.Sprintf("Content-Type: %s; name=\"%s\"\r\n", contentType, encodedName))
		buf.WriteString("Content-Transfer-Encoding: base64\r\n")
		buf.WriteString(fmt.Sprintf("Content-Disposition: attachment; filename=\"%s\"\r\n", encodedName))
		buf.WriteString("\r\n")

		// Stream base64 encode with line wrapping (76 chars per line per RFC 2045)
		lineWriter := &base64LineWriter{w: &buf, lineLen: 76}
		encoder := base64.NewEncoder(base64.StdEncoding, lineWriter)
		_, err = copyBuffer(encoder, file)
		file.Close()
		if err != nil {
			return nil, fmt.Errorf("failed to encode attachment %s: %w", att.Name, err)
		}
		encoder.Close()
		// Ensure we end with CRLF after the base64 content
		if lineWriter.col > 0 {
			buf.WriteString("\r\n")
		}
	}

	// Write closing boundary
	buf.WriteString(fmt.Sprintf("--%s--\r\n", boundary))

	return buf.Bytes(), nil
}

// randomBoundary generates a cryptographically random string for the MIME boundary
func randomBoundary() string {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		// Fallback to PID-based boundary if crypto/rand fails
		return fmt.Sprintf("%d", os.Getpid())
	}
	return fmt.Sprintf("%x", b)
}

// base64LineWriter wraps a writer to insert CRLF line breaks for base64 output
type base64LineWriter struct {
	w       *bytes.Buffer
	lineLen int
	col     int
}

func (lw *base64LineWriter) Write(p []byte) (n int, err error) {
	for _, b := range p {
		if lw.col >= lw.lineLen {
			lw.w.WriteString("\r\n")
			lw.col = 0
		}
		lw.w.WriteByte(b)
		lw.col++
		n++
	}
	return n, nil
}

// copyBuffer copies from src to dst using a fixed buffer to limit memory usage
func copyBuffer(dst io.Writer, src io.Reader) (written int64, err error) {
	buf := make([]byte, 32*1024) // 32KB buffer
	for {
		nr, er := src.Read(buf)
		if nr > 0 {
			nw, ew := dst.Write(buf[0:nr])
			if nw > 0 {
				written += int64(nw)
			}
			if ew != nil {
				err = ew
				break
			}
		}
		if er != nil {
			if er != io.EOF {
				err = er
			}
			break
		}
	}
	return written, err
}
