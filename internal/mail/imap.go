package mail

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"mime/quotedprintable"
	"strings"
	"time"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
	"github.com/emersion/go-message/mail"
	_ "github.com/emersion/go-message/charset" // Register charset decoders

	"maily/internal/auth"
)

// ErrEmailNotFound is returned when an email no longer exists on the server
// (e.g., deleted from another device)
var ErrEmailNotFound = errors.New("email not found on server")

type IMAPClient struct {
	client *imapclient.Client
	creds  *auth.Credentials
}

// Attachment represents email attachment metadata
type Attachment struct {
	PartID      string
	Filename    string
	ContentType string
	Size        int64
	Encoding    string // e.g., "base64", "quoted-printable"
}

type Email struct {
	UID          imap.UID
	MessageID    string
	InternalDate time.Time    // Server receive time (for ordering and cleanup)
	From         string
	ReplyTo      string       // Reply-To address (if different from From)
	To           string
	Subject      string
	Date         time.Time
	Snippet      string
	BodyHTML     string       // HTML body content
	Unread       bool
	References   string       // For threading
	Attachments  []Attachment // Attachment metadata (content fetched on demand)
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

// MailboxInfo contains mailbox metadata
type MailboxInfo struct {
	UIDValidity uint32
	NumMessages uint32
}

// SelectMailboxWithInfo selects a mailbox and returns metadata
func (c *IMAPClient) SelectMailboxWithInfo(name string) (*MailboxInfo, error) {
	mbox, err := c.client.Select(name, nil).Wait()
	if err != nil {
		return nil, err
	}
	return &MailboxInfo{
		UIDValidity: mbox.UIDValidity,
		NumMessages: mbox.NumMessages,
	}, nil
}

// FetchUIDsAndFlags fetches UIDs and flags for emails since the given date
// Returns a map of UID -> unread status
func (c *IMAPClient) FetchUIDsAndFlags(mailbox string, since time.Time) (map[imap.UID]bool, error) {
	_, err := c.client.Select(mailbox, nil).Wait()
	if err != nil {
		return nil, fmt.Errorf("failed to select mailbox: %w", err)
	}

	// Search for emails since the given date
	criteria := &imap.SearchCriteria{
		Since: since,
	}

	searchData, err := c.client.Search(criteria, nil).Wait()
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	if len(searchData.AllSeqNums()) == 0 {
		return make(map[imap.UID]bool), nil
	}

	// Fetch UIDs and flags for found messages
	seqSet := imap.SeqSet{}
	for _, seqNum := range searchData.AllSeqNums() {
		seqSet.AddNum(seqNum)
	}

	fetchOptions := &imap.FetchOptions{
		UID:   true,
		Flags: true,
	}

	messages, err := c.client.Fetch(seqSet, fetchOptions).Collect()
	if err != nil {
		return nil, fmt.Errorf("fetch failed: %w", err)
	}

	result := make(map[imap.UID]bool)
	for _, msg := range messages {
		unread := true
		for _, flag := range msg.Flags {
			if flag == imap.FlagSeen {
				unread = false
				break
			}
		}
		result[msg.UID] = unread
	}

	return result, nil
}

// FetchEmailBody fetches just the body content for a single email by UID
func (c *IMAPClient) FetchEmailBody(mailbox string, uid imap.UID) (bodyHTML string, snippet string, err error) {
	_, err = c.client.Select(mailbox, nil).Wait()
	if err != nil {
		return "", "", fmt.Errorf("failed to select mailbox: %w", err)
	}

	uidSet := imap.UIDSet{}
	uidSet.AddNum(uid)

	fetchOptions := &imap.FetchOptions{
		BodySection: []*imap.FetchItemBodySection{{Peek: true}},
	}

	messages, err := c.client.Fetch(uidSet, fetchOptions).Collect()
	if err != nil {
		return "", "", fmt.Errorf("failed to fetch body: %w", err)
	}

	if len(messages) == 0 {
		return "", "", ErrEmailNotFound
	}

	msg := messages[0]
	if len(msg.BodySection) > 0 {
		bodyHTML, snippet = c.parseBody(msg.BodySection[0].Bytes)
	}

	return bodyHTML, snippet, nil
}

// FetchMessagesByUIDs fetches full messages by their UIDs
func (c *IMAPClient) FetchMessagesByUIDs(mailbox string, uids []imap.UID) ([]Email, error) {
	if len(uids) == 0 {
		return []Email{}, nil
	}

	_, err := c.client.Select(mailbox, nil).Wait()
	if err != nil {
		return nil, fmt.Errorf("failed to select mailbox: %w", err)
	}

	uidSet := imap.UIDSet{}
	for _, uid := range uids {
		uidSet.AddNum(uid)
	}

	fetchOptions := &imap.FetchOptions{
		UID:           true,
		Flags:         true,
		Envelope:      true,
		InternalDate:  true,
		BodyStructure: &imap.FetchItemBodyStructure{Extended: true},
		BodySection:   []*imap.FetchItemBodySection{{Peek: true}},
	}

	messages, err := c.client.Fetch(uidSet, fetchOptions).Collect()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch messages: %w", err)
	}

	emails := make([]Email, 0, len(messages))
	for _, msg := range messages {
		email := c.parseMessage(msg)
		emails = append(emails, email)
	}

	return emails, nil
}

// FetchMessagesByUIDsMetadata fetches metadata only (no body) for given UIDs
func (c *IMAPClient) FetchMessagesByUIDsMetadata(mailbox string, uids []imap.UID) ([]Email, error) {
	if len(uids) == 0 {
		return []Email{}, nil
	}

	_, err := c.client.Select(mailbox, nil).Wait()
	if err != nil {
		return nil, fmt.Errorf("failed to select mailbox: %w", err)
	}

	uidSet := imap.UIDSet{}
	for _, uid := range uids {
		uidSet.AddNum(uid)
	}

	// Metadata only, no body
	fetchOptions := &imap.FetchOptions{
		UID:           true,
		Flags:         true,
		Envelope:      true,
		InternalDate:  true,
		BodyStructure: &imap.FetchItemBodyStructure{Extended: true},
	}

	messages, err := c.client.Fetch(uidSet, fetchOptions).Collect()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch messages: %w", err)
	}

	emails := make([]Email, 0, len(messages))
	for _, msg := range messages {
		email := c.parseMessageMetadata(msg)
		emails = append(emails, email)
	}

	return emails, nil
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
		UID:           true,
		Flags:         true,
		Envelope:      true,
		InternalDate:  true,
		BodyStructure: &imap.FetchItemBodyStructure{Extended: true},
		BodySection:   []*imap.FetchItemBodySection{{Peek: true}},
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

// FetchMessagesMetadata fetches email metadata without body content (fast for slow servers)
func (c *IMAPClient) FetchMessagesMetadata(mailbox string, limit uint32) ([]Email, error) {
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

	// Only fetch metadata, not body content - much faster for slow servers
	fetchOptions := &imap.FetchOptions{
		UID:           true,
		Flags:         true,
		Envelope:      true,
		InternalDate:  true,
		BodyStructure: &imap.FetchItemBodyStructure{Extended: true},
		// No BodySection - body will be fetched on-demand
	}

	messages, err := c.client.Fetch(seqSet, fetchOptions).Collect()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch messages: %w", err)
	}

	emails := make([]Email, 0, len(messages))
	for i := len(messages) - 1; i >= 0; i-- {
		msg := messages[i]
		email := c.parseMessageMetadata(msg)
		emails = append(emails, email)
	}

	return emails, nil
}

// parseMessageMetadata parses message without body content
func (c *IMAPClient) parseMessageMetadata(msg *imapclient.FetchMessageBuffer) Email {
	email := Email{}

	email.UID = msg.UID
	email.InternalDate = msg.InternalDate

	// Parse attachments from BODYSTRUCTURE
	if msg.BodyStructure != nil {
		email.Attachments = c.parseAttachments(msg.BodyStructure, "")
	}

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

	// Body and BodyHTML are empty - will be fetched on-demand
	return email
}

// FetchMessagesSince fetches emails since the given date, up to limit
func (c *IMAPClient) FetchMessagesSince(mailbox string, since time.Time, limit uint32) ([]Email, error) {
	// First get UIDs for emails since the date
	uidMap, err := c.FetchUIDsAndFlags(mailbox, since)
	if err != nil {
		return nil, err
	}

	if len(uidMap) == 0 {
		return []Email{}, nil
	}

	// Convert map keys to slice
	uids := make([]imap.UID, 0, len(uidMap))
	for uid := range uidMap {
		uids = append(uids, uid)
	}

	// Apply limit if needed
	if uint32(len(uids)) > limit {
		// Sort UIDs descending (higher UID = newer) and take top N
		for i := 0; i < len(uids)-1; i++ {
			for j := i + 1; j < len(uids); j++ {
				if uids[j] > uids[i] {
					uids[i], uids[j] = uids[j], uids[i]
				}
			}
		}
		uids = uids[:limit]
	}

	// Fetch full messages for these UIDs
	emails, err := c.FetchMessagesByUIDs(mailbox, uids)
	if err != nil {
		return nil, err
	}

	// Sort by InternalDate descending (newest first)
	for i := 0; i < len(emails)-1; i++ {
		for j := i + 1; j < len(emails); j++ {
			if emails[j].InternalDate.After(emails[i].InternalDate) {
				emails[i], emails[j] = emails[j], emails[i]
			}
		}
	}

	return emails, nil
}

func (c *IMAPClient) parseMessage(msg *imapclient.FetchMessageBuffer) Email {
	email := Email{}

	email.UID = msg.UID
	email.InternalDate = msg.InternalDate

	// Parse attachments from BODYSTRUCTURE
	if msg.BodyStructure != nil {
		email.Attachments = c.parseAttachments(msg.BodyStructure, "")
	}

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
		bodyHTML, snippet := c.parseBody(msg.BodySection[0].Bytes)
		email.BodyHTML = bodyHTML
		email.Snippet = snippet
	}

	return email
}

func (c *IMAPClient) parseBody(body []byte) (string, string) {
	mr, err := mail.CreateReader(strings.NewReader(string(body)))
	if err != nil {
		return string(body), truncateSnippet(stripHTML(string(body)))
	}

	var htmlBody string
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
			if strings.HasPrefix(contentType, "text/html") {
				b, _ := io.ReadAll(part.Body)
				htmlBody = string(b)
			} else if strings.HasPrefix(contentType, "text/plain") {
				b, _ := io.ReadAll(part.Body)
				textBody = string(b)
			}
		}
	}

	// Prefer HTML, fall back to plain text wrapped in <pre>
	if htmlBody != "" {
		snippet := truncateSnippet(stripHTML(htmlBody))
		return htmlBody, snippet
	}
	if textBody != "" {
		// Wrap plain text in pre tag for proper rendering
		htmlBody = "<pre style=\"white-space: pre-wrap; font-family: inherit;\">" + escapeHTML(textBody) + "</pre>"
		return htmlBody, truncateSnippet(textBody)
	}

	// Fallback to raw body
	return string(body), truncateSnippet(stripHTML(string(body)))
}

func escapeHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	return s
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

// parseAttachments extracts attachment metadata from BODYSTRUCTURE
func (c *IMAPClient) parseAttachments(bs imap.BodyStructure, partID string) []Attachment {
	var attachments []Attachment

	switch b := bs.(type) {
	case *imap.BodyStructureSinglePart:
		// Check if this is an attachment
		disposition := ""
		filename := ""

		// Use the Filename() helper method which checks both disposition and params
		filename = b.Filename()

		// Get disposition value
		if disp := b.Disposition(); disp != nil {
			disposition = strings.ToLower(disp.Value)
		}

		// Consider it an attachment if it has a disposition of "attachment"
		// or has a filename and is not text/plain or text/html inline
		isAttachment := disposition == "attachment"
		if !isAttachment && filename != "" {
			contentType := strings.ToLower(b.Type + "/" + b.Subtype)
			if contentType != "text/plain" && contentType != "text/html" {
				isAttachment = true
			}
		}

		if isAttachment && filename != "" {
			att := Attachment{
				PartID:      partID,
				Filename:    filename,
				ContentType: b.Type + "/" + b.Subtype,
				Size:        int64(b.Size),
				Encoding:    strings.ToLower(b.Encoding),
			}
			attachments = append(attachments, att)
		}

	case *imap.BodyStructureMultiPart:
		// Recurse into multipart
		for i, part := range b.Children {
			childPartID := fmt.Sprintf("%d", i+1)
			if partID != "" {
				childPartID = partID + "." + childPartID
			}
			attachments = append(attachments, c.parseAttachments(part, childPartID)...)
		}
	}

	return attachments
}

// FetchAttachment fetches the content of an attachment by its part ID and decodes it
func (c *IMAPClient) FetchAttachment(mailbox string, uid imap.UID, partID string, encoding string) ([]byte, error) {
	_, err := c.client.Select(mailbox, nil).Wait()
	if err != nil {
		return nil, fmt.Errorf("failed to select mailbox: %w", err)
	}

	uidSet := imap.UIDSet{}
	uidSet.AddNum(uid)

	// Parse part ID into section path (e.g., "1.2" -> [1, 2])
	var part []int
	if partID != "" {
		parts := strings.Split(partID, ".")
		for _, p := range parts {
			var num int
			fmt.Sscanf(p, "%d", &num)
			part = append(part, num)
		}
	}

	fetchOptions := &imap.FetchOptions{
		BodySection: []*imap.FetchItemBodySection{
			{Part: part},
		},
	}

	messages, err := c.client.Fetch(uidSet, fetchOptions).Collect()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch attachment: %w", err)
	}

	if len(messages) == 0 {
		return nil, fmt.Errorf("message not found")
	}

	msg := messages[0]
	if len(msg.BodySection) == 0 {
		return nil, fmt.Errorf("attachment not found")
	}

	rawContent := msg.BodySection[0].Bytes

	// Decode based on encoding
	switch strings.ToLower(encoding) {
	case "base64":
		return decodeBase64(rawContent)
	case "quoted-printable":
		reader := quotedprintable.NewReader(strings.NewReader(string(rawContent)))
		decoded, err := io.ReadAll(reader)
		if err != nil {
			return nil, fmt.Errorf("failed to decode quoted-printable: %w", err)
		}
		return decoded, nil
	default:
		// No encoding specified - try to detect base64
		// Base64 content typically only contains A-Za-z0-9+/= and whitespace
		if looksLikeBase64(rawContent) {
			decoded, err := decodeBase64(rawContent)
			if err == nil {
				return decoded, nil
			}
		}
		// Return as-is
		return rawContent, nil
	}
}

// looksLikeBase64 checks if content appears to be base64 encoded
func looksLikeBase64(content []byte) bool {
	if len(content) < 20 {
		return false
	}
	// Check first 100 non-whitespace chars for base64 alphabet
	checked := 0
	for _, b := range content {
		if b == ' ' || b == '\t' || b == '\n' || b == '\r' {
			continue
		}
		if !((b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z') || (b >= '0' && b <= '9') || b == '+' || b == '/' || b == '=') {
			return false
		}
		checked++
		if checked >= 100 {
			break
		}
	}
	return checked >= 20
}

// decodeBase64 decodes base64 content with whitespace handling
func decodeBase64(rawContent []byte) ([]byte, error) {
	// Strip all whitespace
	cleaned := strings.Map(func(r rune) rune {
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			return -1
		}
		return r
	}, string(rawContent))

	decoded, err := base64.StdEncoding.DecodeString(cleaned)
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64: %w", err)
	}
	return decoded, nil
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

func (c *IMAPClient) MoveToTrash(uids []imap.UID) error {
	return c.MoveToTrashFromMailbox(uids, "INBOX")
}

func (c *IMAPClient) MoveToTrashFromMailbox(uids []imap.UID, mailbox string) error {
	if len(uids) == 0 {
		return nil
	}

	// Find trash folder (this may invalidate mailbox selection on some servers like Yahoo)
	trashFolder, err := c.findTrashFolder()
	if err != nil {
		return fmt.Errorf("failed to find trash folder: %w", err)
	}

	// Re-select mailbox before Move (required after List on some servers)
	if err := c.SelectMailbox(mailbox); err != nil {
		return fmt.Errorf("failed to select mailbox: %w", err)
	}

	uidSet := imap.UIDSet{}
	for _, uid := range uids {
		uidSet.AddNum(uid)
	}

	// Move to trash
	if _, err := c.client.Move(uidSet, trashFolder).Wait(); err != nil {
		return err
	}

	return nil
}

func (c *IMAPClient) findTrashFolder() (string, error) {
	// Try Gmail-specific trash folder first
	if c.mailboxExists(GmailTrash) {
		return GmailTrash, nil
	}

	// Try to find folder with \Trash special-use attribute
	listCmd := c.client.List("", "*", &imap.ListOptions{
		ReturnStatus: &imap.StatusOptions{},
	})
	defer listCmd.Close()

	for {
		mbox := listCmd.Next()
		if mbox == nil {
			break
		}
		for _, attr := range mbox.Attrs {
			if attr == imap.MailboxAttrTrash {
				return mbox.Mailbox, nil
			}
		}
	}

	// Fallback to common trash folder names
	fallbacks := []string{"Trash", "Deleted", "Deleted Items", "Deleted Messages"}
	for _, name := range fallbacks {
		if c.mailboxExists(name) {
			return name, nil
		}
	}

	return "", fmt.Errorf("trash folder not found")
}

func (c *IMAPClient) mailboxExists(name string) bool {
	listCmd := c.client.List("", name, nil)
	defer listCmd.Close()
	return listCmd.Next() != nil
}

func (c *IMAPClient) findArchiveFolder() (string, error) {
	// Try Gmail-specific archive folder first
	if c.mailboxExists(GmailAllMail) {
		return GmailAllMail, nil
	}

	// Try to find folder with \Archive special-use attribute
	listCmd := c.client.List("", "*", &imap.ListOptions{
		ReturnStatus: &imap.StatusOptions{},
	})
	defer listCmd.Close()

	for {
		mbox := listCmd.Next()
		if mbox == nil {
			break
		}
		for _, attr := range mbox.Attrs {
			if attr == imap.MailboxAttrArchive {
				return mbox.Mailbox, nil
			}
		}
	}

	// Fallback to common archive folder names
	fallbacks := []string{"Archive", "All Mail"}
	for _, name := range fallbacks {
		if c.mailboxExists(name) {
			return name, nil
		}
	}

	return "", fmt.Errorf("archive folder not found")
}

func (c *IMAPClient) findDraftsFolder() (string, error) {
	// Try Gmail-specific drafts folder first
	if c.mailboxExists(GmailDrafts) {
		return GmailDrafts, nil
	}

	// Try to find folder with \Drafts special-use attribute
	listCmd := c.client.List("", "*", &imap.ListOptions{
		ReturnStatus: &imap.StatusOptions{},
	})
	defer listCmd.Close()

	for {
		mbox := listCmd.Next()
		if mbox == nil {
			break
		}
		for _, attr := range mbox.Attrs {
			if attr == imap.MailboxAttrDrafts {
				return mbox.Mailbox, nil
			}
		}
	}

	// Fallback to common drafts folder names
	fallbacks := []string{"Drafts", "Draft"}
	for _, name := range fallbacks {
		if c.mailboxExists(name) {
			return name, nil
		}
	}

	return "", fmt.Errorf("drafts folder not found")
}

func (c *IMAPClient) ArchiveMessages(uids []imap.UID) error {
	if len(uids) == 0 {
		return nil
	}

	archiveFolder, err := c.findArchiveFolder()
	if err != nil {
		return err
	}

	uidSet := imap.UIDSet{}
	for _, uid := range uids {
		uidSet.AddNum(uid)
	}

	if _, err := c.client.Move(uidSet, archiveFolder).Wait(); err != nil {
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
	draftsFolder, err := c.findDraftsFolder()
	if err != nil {
		return err
	}

	// Build the email message
	msg := fmt.Sprintf("From: %s\r\n"+
		"To: %s\r\n"+
		"Subject: %s\r\n"+
		"MIME-Version: 1.0\r\n"+
		"Content-Type: text/plain; charset=\"utf-8\"\r\n"+
		"\r\n"+
		"%s", c.creds.Email, to, subject, body)

	// Append to Drafts folder with Draft flag
	appendCmd := c.client.Append(draftsFolder, int64(len(msg)), nil)
	if _, err := appendCmd.Write([]byte(msg)); err != nil {
		return fmt.Errorf("failed to write draft: %w", err)
	}
	if err := appendCmd.Close(); err != nil {
		return fmt.Errorf("failed to save draft: %w", err)
	}
	return nil
}

// SearchMessages searches for emails
// For Gmail, uses X-GM-RAW extension with full search syntax
// For other providers, uses standard IMAP TEXT search
func (c *IMAPClient) SearchMessages(mailbox string, query string) ([]Email, error) {
	// Use provider-appropriate search method
	uids, err := Search(c.creds, mailbox, query)
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
		UID:           true,
		Flags:         true,
		Envelope:      true,
		InternalDate:  true,
		BodyStructure: &imap.FetchItemBodyStructure{Extended: true},
		BodySection:   []*imap.FetchItemBodySection{{Peek: true}},
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
			bodyHTML, snippet := c.parseBody(msg.BodySection[0].Bytes)
			email.BodyHTML = bodyHTML
			email.Snippet = snippet
		}
		emails = append(emails, email)
	}

	return emails, nil
}

// FetchByUIDs fetches emails by their UIDs (used for paginated search results)
func (c *IMAPClient) FetchByUIDs(mailbox string, uids []imap.UID) ([]Email, error) {
	if len(uids) == 0 {
		return []Email{}, nil
	}

	// Select mailbox for fetching
	_, err := c.client.Select(mailbox, nil).Wait()
	if err != nil {
		return nil, fmt.Errorf("failed to select mailbox: %w", err)
	}

	// Build UID set
	uidSet := imap.UIDSet{}
	for _, uid := range uids {
		uidSet.AddNum(uid)
	}

	fetchOptions := &imap.FetchOptions{
		UID:           true,
		Flags:         true,
		Envelope:      true,
		InternalDate:  true,
		BodyStructure: &imap.FetchItemBodyStructure{Extended: true},
		BodySection:   []*imap.FetchItemBodySection{{Peek: true}},
	}

	messages, err := c.client.Fetch(uidSet, fetchOptions).Collect()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch messages: %w", err)
	}

	emails := make([]Email, 0, len(messages))
	for i := len(messages) - 1; i >= 0; i-- {
		msg := messages[i]
		email := c.parseMessageHeader(msg)
		// Parse body for snippet
		if len(msg.BodySection) > 0 {
			_, snippet := c.parseBody(msg.BodySection[0].Bytes)
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
	email.InternalDate = msg.InternalDate

	// Parse attachments from BODYSTRUCTURE
	if msg.BodyStructure != nil {
		email.Attachments = c.parseAttachments(msg.BodyStructure, "")
	}

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
