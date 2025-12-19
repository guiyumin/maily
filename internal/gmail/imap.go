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
	UID     imap.UID
	From    string
	To      string
	Subject string
	Date    time.Time
	Snippet string
	Body    string
	Unread  bool
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

		if len(env.From) > 0 {
			from := env.From[0]
			if from.Name != "" {
				email.From = fmt.Sprintf("%s <%s@%s>", from.Name, from.Mailbox, from.Host)
			} else {
				email.From = fmt.Sprintf("%s@%s", from.Mailbox, from.Host)
			}
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
	var result strings.Builder
	inTag := false
	for _, r := range html {
		if r == '<' {
			inTag = true
		} else if r == '>' {
			inTag = false
		} else if !inTag {
			result.WriteRune(r)
		}
	}
	return strings.TrimSpace(result.String())
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
