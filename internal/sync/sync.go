package sync

import (
	"fmt"
	"time"

	"github.com/emersion/go-imap/v2"

	"maily/internal/auth"
	"maily/internal/cache"
	"maily/internal/mail"
)

const (
	// SyncDays is the number of days to sync
	SyncDays = 14
	// QuickRefreshLimit is the number of emails to fetch for quick refresh
	QuickRefreshLimit = 50
)

// Syncer handles email synchronization
type Syncer struct {
	cache   *cache.Cache
	account *auth.Account
}

// NewSyncer creates a new syncer for an account
func NewSyncer(c *cache.Cache, account *auth.Account) *Syncer {
	return &Syncer{
		cache:   c,
		account: account,
	}
}

// FullSync performs a full sync for a mailbox (14 days)
func (s *Syncer) FullSync(mailbox string) error {
	email := s.account.Credentials.Email

	// Try to acquire lock
	acquired, err := s.cache.AcquireLock(email)
	if err != nil {
		return fmt.Errorf("failed to acquire lock: %w", err)
	}
	if !acquired {
		return fmt.Errorf("sync already in progress")
	}
	defer s.cache.ReleaseLock(email)

	// Connect to IMAP
	client, err := mail.NewIMAPClient(&s.account.Credentials)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer client.Close()

	// Get mailbox info for UIDVALIDITY
	info, err := client.SelectMailboxWithInfo(mailbox)
	if err != nil {
		return fmt.Errorf("failed to select mailbox: %w", err)
	}

	// Check UIDVALIDITY
	meta, err := s.cache.LoadMetadata(email, mailbox)
	if err != nil {
		return fmt.Errorf("failed to load metadata: %w", err)
	}

	if meta != nil && meta.UIDValidity != info.UIDValidity {
		// UIDVALIDITY changed, invalidate cache
		if err := s.cache.InvalidateMailbox(email, mailbox); err != nil {
			return fmt.Errorf("failed to invalidate cache: %w", err)
		}
		meta = nil
	}

	// Fetch UIDs and flags for last 14 days
	since := time.Now().AddDate(0, 0, -SyncDays)
	serverUIDs, err := client.FetchUIDsAndFlags(mailbox, since)
	if err != nil {
		return fmt.Errorf("failed to fetch UIDs: %w", err)
	}

	// Get cached UIDs
	cachedUIDs, err := s.cache.GetCachedUIDs(email, mailbox)
	if err != nil {
		return fmt.Errorf("failed to get cached UIDs: %w", err)
	}

	// Find new UIDs (on server but not in cache)
	var newUIDs []imap.UID
	for uid := range serverUIDs {
		if !cachedUIDs[uid] {
			newUIDs = append(newUIDs, uid)
		}
	}

	// Find deleted UIDs (in cache but not on server)
	for uid := range cachedUIDs {
		if _, ok := serverUIDs[uid]; !ok {
			if err := s.cache.DeleteEmail(email, mailbox, uid); err != nil {
				// Log but don't fail
				continue
			}
		}
	}

	// Fetch new emails
	if len(newUIDs) > 0 {
		emails, err := client.FetchMessagesByUIDs(mailbox, newUIDs)
		if err != nil {
			return fmt.Errorf("failed to fetch new emails: %w", err)
		}

		for _, e := range emails {
			cached := emailToCached(e)
			if err := s.cache.SaveEmail(email, mailbox, cached); err != nil {
				// Log but don't fail
				continue
			}
		}
	}

	// Update flags for existing emails
	for uid, unread := range serverUIDs {
		if cachedUIDs[uid] {
			// Check if flag changed
			if err := s.cache.UpdateEmailFlags(email, mailbox, uid, unread); err != nil {
				// Log but don't fail
				continue
			}
		}
	}

	// Cleanup old emails
	olderThan := time.Now().AddDate(0, 0, -SyncDays)
	s.cache.Cleanup(email, mailbox, olderThan)

	// Update metadata
	newMeta := &cache.Metadata{
		UIDValidity: info.UIDValidity,
		LastSync:    time.Now(),
	}
	if err := s.cache.SaveMetadata(email, mailbox, newMeta); err != nil {
		return fmt.Errorf("failed to save metadata: %w", err)
	}

	return nil
}

// QuickRefresh fetches the latest 50 emails only
func (s *Syncer) QuickRefresh(mailbox string) error {
	email := s.account.Credentials.Email

	// Try to acquire lock
	acquired, err := s.cache.AcquireLock(email)
	if err != nil {
		return fmt.Errorf("failed to acquire lock: %w", err)
	}
	if !acquired {
		return fmt.Errorf("sync already in progress")
	}
	defer s.cache.ReleaseLock(email)

	// Connect to IMAP
	client, err := mail.NewIMAPClient(&s.account.Credentials)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer client.Close()

	// Fetch latest 50 emails
	emails, err := client.FetchMessages(mailbox, QuickRefreshLimit)
	if err != nil {
		return fmt.Errorf("failed to fetch emails: %w", err)
	}

	// Save to cache
	for _, e := range emails {
		cached := emailToCached(e)
		if err := s.cache.SaveEmail(email, mailbox, cached); err != nil {
			// Log but don't fail
			continue
		}
	}

	// Update metadata
	info, err := client.SelectMailboxWithInfo(mailbox)
	if err == nil {
		meta := &cache.Metadata{
			UIDValidity: info.UIDValidity,
			LastSync:    time.Now(),
		}
		s.cache.SaveMetadata(email, mailbox, meta)
	}

	return nil
}

// emailToCached converts a mail.Email to cache.CachedEmail
func emailToCached(e mail.Email) cache.CachedEmail {
	attachments := make([]cache.Attachment, len(e.Attachments))
	for i, a := range e.Attachments {
		attachments[i] = cache.Attachment{
			PartID:      a.PartID,
			Filename:    a.Filename,
			ContentType: a.ContentType,
			Size:        a.Size,
		}
	}

	return cache.CachedEmail{
		UID:          e.UID,
		MessageID:    e.MessageID,
		InternalDate: e.InternalDate,
		From:         e.From,
		ReplyTo:      e.ReplyTo,
		To:           e.To,
		Subject:      e.Subject,
		Date:         e.Date,
		Snippet:      e.Snippet,
		Body:         e.Body,
		Unread:       e.Unread,
		References:   e.References,
		Attachments:  attachments,
	}
}
