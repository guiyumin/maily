package gmail

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
	"github.com/emersion/go-message/mail"
	_ "github.com/emersion/go-message/charset" // Register charset decoders

	"maily/internal/auth"
)

type IMAPClient struct {
	client *imapclient.Client
	creds  *auth.Credentials
}

type Email struct {
	UID        imap.UID
	MessageID  string
	From       string
	ReplyTo    string // Reply-To address (if different from From)
	To         string
	Subject    string
	Date       time.Time
	Snippet    string
	Body       string
	Unread     bool
	References string // For threading
}

func NewIMAPClient(creds *auth.Credentials) (*IMAPClient, error) {
	addr := fmt.Sprintf("%s:%d", creds.IMAPHost, creds.IMAPPort)

	client, err := imapclient.DialTLS(addr, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to IMAP server: %w", err)
	}

	if err := client.Login(creds.Email, creds.Password).Wait(); err != nil {
		client.Close()
		return nil, fmt.Errorf("login failed: %w", err)
	}

	return &IMAPClient{
		client: client,
		creds:  creds,
	}, nil
}

func (c *IMAPClient) Close() error {
	if c.client != nil {
		return c.client.Close()
	}
	return nil
}

func (c *IMAPClient) ListMailboxes() ([]string, error) {
	mailboxes, err := c.client.List("", "*", nil).Collect()
	if err != nil {
		return nil, err
	}

	names := make([]string, len(mailboxes))
	for i, mbox := range mailboxes {
		names[i] = mbox.Mailbox
	}
	return names, nil
}

func (c *IMAPClient) SelectMailbox(name string) error {
	_, err := c.client.Select(name, nil).Wait()
	return err
}

func (c *IMAPClient) FetchMessages(mailbox string, limit uint32) ([]Email, error) {
	mbox, err := c.client.Select(mailbox, nil).Wait()
	if err != nil {
		return nil, fmt.Errorf("failed to select mailbox: %w", err)
	}

	if mbox.NumMessages == 0 {
		return []Email{}, nil
	}

	from := uint32(1)
	if mbox.NumMessages > limit {
		from = mbox.NumMessages - limit + 1
	}

	seqSet := imap.SeqSet{}
	seqSet.AddRange(from, mbox.NumMessages)

	fetchOptions := &imap.FetchOptions{
		UID:         true,
		Flags:       true,
		Envelope:    true,
		BodySection: []*imap.FetchItemBodySection{{Peek: true}},
	}

	messages, err := c.client.Fetch(seqSet, fetchOptions).Collect()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch messages: %w", err)
	}

	emails := make([]Email, 0, len(messages))
	for i := len(messages) - 1; i >= 0; i-- {
		msg := messages[i]
		email := c.parseMessage(msg)
		emails = append(emails, email)
	}

	return emails, nil
}

func (c *IMAPClient) parseMessage(msg *imapclient.FetchMessageBuffer) Email {
	email := Email{}

	email.UID = msg.UID

	if env := msg.Envelope; env != nil {
		email.Subject = env.Subject
		email.Date = env.Date
		email.MessageID = env.MessageID
		if len(env.InReplyTo) > 0 {
			email.References = strings.Join(env.InReplyTo, " ")
		}

		if len(env.From) > 0 {
			from := env.From[0]
			if from.Name != "" {
				email.From = fmt.Sprintf("%s <%s@%s>", from.Name, from.Mailbox, from.Host)
			} else {
				email.From = fmt.Sprintf("%s@%s", from.Mailbox, from.Host)
			}
		}

		// ReplyTo field (use if different from From)
		if len(env.ReplyTo) > 0 {
			replyTo := env.ReplyTo[0]
			email.ReplyTo = fmt.Sprintf("%s@%s", replyTo.Mailbox, replyTo.Host)
		}

		if len(env.To) > 0 {
			to := env.To[0]
			if to.Name != "" {
				email.To = fmt.Sprintf("%s <%s@%s>", to.Name, to.Mailbox, to.Host)
			} else {
				email.To = fmt.Sprintf("%s@%s", to.Mailbox, to.Host)
			}
		}
	}

	email.Unread = true
	for _, flag := range msg.Flags {
		if flag == imap.FlagSeen {
			email.Unread = false
			break
		}
	}

	if len(msg.BodySection) > 0 {
		body, snippet := c.parseBody(msg.BodySection[0].Bytes)
		email.Body = body
		email.Snippet = snippet
	}

	return email
}

func (c *IMAPClient) parseBody(body []byte) (string, string) {
	mr, err := mail.CreateReader(strings.NewReader(string(body)))
	if err != nil {
		return string(body), truncateSnippet(string(body))
	}

	var textBody string
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}

		switch h := part.Header.(type) {
		case *mail.InlineHeader:
			contentType, _, _ := h.ContentType()
			if strings.HasPrefix(contentType, "text/plain") {
				b, _ := io.ReadAll(part.Body)
				textBody = string(b)
			} else if strings.HasPrefix(contentType, "text/html") && textBody == "" {
				b, _ := io.ReadAll(part.Body)
				textBody = stripHTML(string(b))
			}
		}
	}

	if textBody == "" {
		textBody = string(body)
	}

	return textBody, truncateSnippet(textBody)
}

func truncateSnippet(s string) string {
	snippet := s
	if len(snippet) > 200 {
		snippet = snippet[:200] + "..."
	}
	snippet = strings.ReplaceAll(snippet, "\n", " ")
	snippet = strings.ReplaceAll(snippet, "\r", "")
	return snippet
}

func stripHTML(html string) string {
	// Remove style blocks
	for {
		styleStart := strings.Index(strings.ToLower(html), "<style")
		if styleStart == -1 {
			break
		}
		styleEnd := strings.Index(strings.ToLower(html[styleStart:]), "</style>")
		if styleEnd == -1 {
			html = html[:styleStart]
		} else {
			html = html[:styleStart] + html[styleStart+styleEnd+8:]
		}
	}

	// Remove script blocks
	for {
		scriptStart := strings.Index(strings.ToLower(html), "<script")
		if scriptStart == -1 {
			break
		}
		scriptEnd := strings.Index(strings.ToLower(html[scriptStart:]), "</script>")
		if scriptEnd == -1 {
			html = html[:scriptStart]
		} else {
			html = html[:scriptStart] + html[scriptStart+scriptEnd+9:]
		}
	}

	// Remove HTML tags
	var result strings.Builder
	inTag := false
	for _, r := range html {
		if r == '<' {
			inTag = true
		} else if r == '>' {
			inTag = false
			result.WriteRune(' ') // Replace tag with space
		} else if !inTag {
			result.WriteRune(r)
		}
	}

	text := result.String()

	// Decode common HTML entities
	text = strings.ReplaceAll(text, "&nbsp;", " ")
	text = strings.ReplaceAll(text, "&amp;", "&")
	text = strings.ReplaceAll(text, "&lt;", "<")
	text = strings.ReplaceAll(text, "&gt;", ">")
	text = strings.ReplaceAll(text, "&quot;", "\"")
	text = strings.ReplaceAll(text, "&#39;", "'")

	// Clean up whitespace
	lines := strings.Split(text, "\n")
	var cleanLines []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			cleanLines = append(cleanLines, line)
		}
	}

	return strings.Join(cleanLines, "\n")
}

func (c *IMAPClient) MarkAsRead(uid imap.UID) error {
	uidSet := imap.UIDSet{}
	uidSet.AddNum(uid)

	storeFlags := &imap.StoreFlags{
		Op:    imap.StoreFlagsAdd,
		Flags: []imap.Flag{imap.FlagSeen},
	}

	cmd := c.client.Store(uidSet, storeFlags, nil)
	return cmd.Close()
}

func (c *IMAPClient) MarkAsUnread(uid imap.UID) error {
	uidSet := imap.UIDSet{}
	uidSet.AddNum(uid)

	storeFlags := &imap.StoreFlags{
		Op:    imap.StoreFlagsDel,
		Flags: []imap.Flag{imap.FlagSeen},
	}

	cmd := c.client.Store(uidSet, storeFlags, nil)
	return cmd.Close()
}

func (c *IMAPClient) DeleteMessage(uid imap.UID) error {
	uidSet := imap.UIDSet{}
	uidSet.AddNum(uid)

	storeFlags := &imap.StoreFlags{
		Op:    imap.StoreFlagsAdd,
		Flags: []imap.Flag{imap.FlagDeleted},
	}

	if err := c.client.Store(uidSet, storeFlags, nil).Close(); err != nil {
		return err
	}

	return c.client.Expunge().Close()
}

func (c *IMAPClient) DeleteMessages(uids []imap.UID) error {
	if len(uids) == 0 {
		return nil
	}

	uidSet := imap.UIDSet{}
	for _, uid := range uids {
		uidSet.AddNum(uid)
	}

	storeFlags := &imap.StoreFlags{
		Op:    imap.StoreFlagsAdd,
		Flags: []imap.Flag{imap.FlagDeleted},
	}

	if err := c.client.Store(uidSet, storeFlags, nil).Close(); err != nil {
		return err
	}

	return c.client.Expunge().Close()
}

func (c *IMAPClient) ArchiveMessages(uids []imap.UID) error {
	if len(uids) == 0 {
		return nil
	}

	uidSet := imap.UIDSet{}
	for _, uid := range uids {
		uidSet.AddNum(uid)
	}

	// Move to All Mail (archive in Gmail)
	if _, err := c.client.Move(uidSet, "[Gmail]/All Mail").Wait(); err != nil {
		return err
	}

	return nil
}

func (c *IMAPClient) MarkMessagesAsRead(uids []imap.UID) error {
	if len(uids) == 0 {
		return nil
	}

	uidSet := imap.UIDSet{}
	for _, uid := range uids {
		uidSet.AddNum(uid)
	}

	storeFlags := &imap.StoreFlags{
		Op:    imap.StoreFlagsAdd,
		Flags: []imap.Flag{imap.FlagSeen},
	}

	return c.client.Store(uidSet, storeFlags, nil).Close()
}

// SaveDraft saves an email to the Drafts folder
func (c *IMAPClient) SaveDraft(to, subject, body string) error {
	// Build the email message
	msg := fmt.Sprintf("From: %s\r\n"+
		"To: %s\r\n"+
		"Subject: %s\r\n"+
		"MIME-Version: 1.0\r\n"+
		"Content-Type: text/plain; charset=\"utf-8\"\r\n"+
		"\r\n"+
		"%s", c.creds.Email, to, subject, body)

	// Append to Drafts folder with Draft flag
	appendCmd := c.client.Append("[Gmail]/Drafts", int64(len(msg)), nil)
	if _, err := appendCmd.Write([]byte(msg)); err != nil {
		return fmt.Errorf("failed to write draft: %w", err)
	}
	if err := appendCmd.Close(); err != nil {
		return fmt.Errorf("failed to save draft: %w", err)
	}
	return nil
}

// SearchMessages searches for emails using Gmail's X-GM-RAW extension
// This supports Gmail's full search syntax (from:, has:attachment, category:, etc.)
func (c *IMAPClient) SearchMessages(mailbox string, query string) ([]Email, error) {
	// Use Gmail's X-GM-RAW for powerful search
	uids, err := GmailSearch(c.creds, mailbox, query)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	if len(uids) == 0 {
		return []Email{}, nil
	}

	// Select mailbox for fetching
	_, err = c.client.Select(mailbox, nil).Wait()
	if err != nil {
		return nil, fmt.Errorf("failed to select mailbox: %w", err)
	}

	// Fetch the found messages
	uidSet := imap.UIDSet{}
	for _, uid := range uids {
		uidSet.AddNum(uid)
	}

	fetchOptions := &imap.FetchOptions{
		UID:         true,
		Flags:       true,
		Envelope:    true,
		BodySection: []*imap.FetchItemBodySection{{Peek: true}},
	}

	messages, err := c.client.Fetch(uidSet, fetchOptions).Collect()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch messages: %w", err)
	}

	emails := make([]Email, 0, len(messages))
	for i := len(messages) - 1; i >= 0; i-- {
		msg := messages[i]
		email := c.parseMessageHeader(msg)
		// Parse body if available
		if len(msg.BodySection) > 0 {
			body, snippet := c.parseBody(msg.BodySection[0].Bytes)
			email.Body = body
			email.Snippet = snippet
		}
		emails = append(emails, email)
	}

	return emails, nil
}

// parseMessageHeader parses message headers without body (faster for search results)
func (c *IMAPClient) parseMessageHeader(msg *imapclient.FetchMessageBuffer) Email {
	email := Email{}

	email.UID = msg.UID

	if env := msg.Envelope; env != nil {
		email.Subject = env.Subject
		email.Date = env.Date
		email.MessageID = env.MessageID
		if len(env.InReplyTo) > 0 {
			email.References = strings.Join(env.InReplyTo, " ")
		}

		if len(env.From) > 0 {
			from := env.From[0]
			if from.Name != "" {
				email.From = fmt.Sprintf("%s <%s@%s>", from.Name, from.Mailbox, from.Host)
			} else {
				email.From = fmt.Sprintf("%s@%s", from.Mailbox, from.Host)
			}
		}

		if len(env.ReplyTo) > 0 {
			replyTo := env.ReplyTo[0]
			email.ReplyTo = fmt.Sprintf("%s@%s", replyTo.Mailbox, replyTo.Host)
		}

		if len(env.To) > 0 {
			to := env.To[0]
			if to.Name != "" {
				email.To = fmt.Sprintf("%s <%s@%s>", to.Name, to.Mailbox, to.Host)
			} else {
				email.To = fmt.Sprintf("%s@%s", to.Mailbox, to.Host)
			}
		}
	}

	email.Unread = true
	for _, flag := range msg.Flags {
		if flag == imap.FlagSeen {
			email.Unread = false
			break
		}
	}

	return email
}
