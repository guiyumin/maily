package cache

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/emersion/go-imap/v2"

	"maily/internal/proc"
)

// Attachment represents email attachment metadata
type Attachment struct {
	PartID      string `json:"part_id"`
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	Size        int64  `json:"size"`
	Encoding    string `json:"encoding,omitempty"`
}

// CachedEmail represents an email stored in the cache
type CachedEmail struct {
	UID          imap.UID     `json:"uid"`
	MessageID    string       `json:"message_id"`
	InternalDate time.Time    `json:"internal_date"`
	From         string       `json:"from"`
	ReplyTo      string       `json:"reply_to,omitempty"`
	To           string       `json:"to"`
	Subject      string       `json:"subject"`
	Date         time.Time    `json:"date"`
	Snippet      string       `json:"snippet"`
	Body         string       `json:"body"`
	Unread       bool         `json:"unread"`
	References   string       `json:"references,omitempty"`
	Attachments  []Attachment `json:"attachments,omitempty"`
}

// Metadata tracks mailbox sync state
type Metadata struct {
	UIDValidity uint32    `json:"uidvalidity"`
	LastSync    time.Time `json:"last_sync"`
}

// Cache manages persistent email storage
type Cache struct {
	baseDir string
}

// New creates a new cache instance
func New() (*Cache, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	baseDir := filepath.Join(homeDir, ".config", "maily", "cache")
	return &Cache{baseDir: baseDir}, nil
}

// encodeMailbox URL-encodes a mailbox name for filesystem safety
func encodeMailbox(mailbox string) string {
	return url.PathEscape(mailbox)
}

// getMailboxPath returns the path for a specific account/mailbox
func (c *Cache) getMailboxPath(account, mailbox string) string {
	return filepath.Join(c.baseDir, account, encodeMailbox(mailbox))
}

// getLockPath returns the lock file path for an account
func (c *Cache) getLockPath(account string) string {
	return filepath.Join(c.baseDir, account, ".sync.lock")
}

func lockActive(info proc.LockInfo) bool {
	if info.PID <= 0 {
		return false
	}

	if info.Start != "" {
		start, err := proc.StartTime(info.PID)
		if err == nil && start != "" {
			return start == info.Start
		}
		return proc.Exists(info.PID)
	}

	if proc.IsMailyProcess(info.PID) {
		return true
	}
	return proc.Exists(info.PID)
}

// AcquireLock tries to acquire the sync lock for an account
// Returns true if lock acquired, false if already locked
func (c *Cache) AcquireLock(account string) (bool, error) {
	lockPath := c.getLockPath(account)

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(lockPath), 0700); err != nil {
		return false, err
	}

	// Check if lock exists
	data, err := os.ReadFile(lockPath)
	if err == nil {
		info, err := proc.ParseLockInfo(data)
		if err == nil && lockActive(info) {
			return false, nil
		}
		// Stale or invalid lock, remove it
		os.Remove(lockPath)
	}

	// Create lock file with our PID
	pid := os.Getpid()
	content := fmt.Sprintf("%d", pid)
	if start, err := proc.StartTime(pid); err == nil && start != "" {
		content = fmt.Sprintf("%d:%s", pid, start)
	}
	if err := os.WriteFile(lockPath, []byte(content), 0600); err != nil {
		return false, err
	}

	return true, nil
}

// ReleaseLock releases the sync lock for an account
func (c *Cache) ReleaseLock(account string) error {
	return os.Remove(c.getLockPath(account))
}

// LoadMetadata loads mailbox metadata
func (c *Cache) LoadMetadata(account, mailbox string) (*Metadata, error) {
	path := filepath.Join(c.getMailboxPath(account, mailbox), "metadata.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var meta Metadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, err
	}
	return &meta, nil
}

// SaveMetadata saves mailbox metadata (atomic write)
func (c *Cache) SaveMetadata(account, mailbox string, meta *Metadata) error {
	dir := c.getMailboxPath(account, mailbox)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}

	// Atomic write: write to temp file, then rename
	path := filepath.Join(dir, "metadata.json")
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}

// LoadEmails loads all cached emails for a mailbox, sorted by InternalDate descending
func (c *Cache) LoadEmails(account, mailbox string) ([]CachedEmail, error) {
	dir := c.getMailboxPath(account, mailbox)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var emails []CachedEmail
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") || entry.Name() == "metadata.json" {
			continue
		}

		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}

		var email CachedEmail
		if err := json.Unmarshal(data, &email); err != nil {
			continue
		}
		emails = append(emails, email)
	}

	// Sort by InternalDate descending (newest first)
	sort.Slice(emails, func(i, j int) bool {
		return emails[i].InternalDate.After(emails[j].InternalDate)
	})

	return emails, nil
}

// LoadEmailsLimit loads up to limit emails, sorted by InternalDate descending
func (c *Cache) LoadEmailsLimit(account, mailbox string, limit int) ([]CachedEmail, error) {
	emails, err := c.LoadEmails(account, mailbox)
	if err != nil {
		return nil, err
	}
	if len(emails) > limit {
		return emails[:limit], nil
	}
	return emails, nil
}

// SaveEmail saves a single email to cache (atomic write)
func (c *Cache) SaveEmail(account, mailbox string, email CachedEmail) error {
	dir := c.getMailboxPath(account, mailbox)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	data, err := json.MarshalIndent(email, "", "  ")
	if err != nil {
		return err
	}

	// Filename is UID.json
	filename := fmt.Sprintf("%d.json", email.UID)
	path := filepath.Join(dir, filename)
	tmpPath := path + ".tmp"

	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}

// DeleteEmail deletes an email from cache
func (c *Cache) DeleteEmail(account, mailbox string, uid imap.UID) error {
	path := filepath.Join(c.getMailboxPath(account, mailbox), fmt.Sprintf("%d.json", uid))
	err := os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// GetCachedUIDs returns a set of all cached UIDs for a mailbox
func (c *Cache) GetCachedUIDs(account, mailbox string) (map[imap.UID]bool, error) {
	dir := c.getMailboxPath(account, mailbox)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[imap.UID]bool), nil
		}
		return nil, err
	}

	uids := make(map[imap.UID]bool)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") || entry.Name() == "metadata.json" {
			continue
		}

		// Parse UID from filename (e.g., "12345.json")
		name := strings.TrimSuffix(entry.Name(), ".json")
		uid, err := strconv.ParseUint(name, 10, 32)
		if err != nil {
			continue
		}
		uids[imap.UID(uid)] = true
	}

	return uids, nil
}

// Cleanup deletes cached emails older than the given duration
func (c *Cache) Cleanup(account, mailbox string, olderThan time.Time) (int, error) {
	emails, err := c.LoadEmails(account, mailbox)
	if err != nil {
		return 0, err
	}

	deleted := 0
	for _, email := range emails {
		if email.InternalDate.Before(olderThan) {
			if err := c.DeleteEmail(account, mailbox, email.UID); err == nil {
				deleted++
			}
		}
	}

	return deleted, nil
}

// InvalidateMailbox removes all cached emails for a mailbox
func (c *Cache) InvalidateMailbox(account, mailbox string) error {
	dir := c.getMailboxPath(account, mailbox)
	return os.RemoveAll(dir)
}

// GetEmail loads a single email by UID
func (c *Cache) GetEmail(account, mailbox string, uid imap.UID) (*CachedEmail, error) {
	path := filepath.Join(c.getMailboxPath(account, mailbox), fmt.Sprintf("%d.json", uid))
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var email CachedEmail
	if err := json.Unmarshal(data, &email); err != nil {
		return nil, err
	}
	return &email, nil
}

// UpdateEmailFlags updates only the Unread flag of a cached email
func (c *Cache) UpdateEmailFlags(account, mailbox string, uid imap.UID, unread bool) error {
	email, err := c.GetEmail(account, mailbox, uid)
	if err != nil || email == nil {
		return err
	}

	email.Unread = unread
	return c.SaveEmail(account, mailbox, *email)
}

// IsFresh returns true if the cache was synced within the given duration
func (c *Cache) IsFresh(account, mailbox string, maxAge time.Duration) bool {
	meta, err := c.LoadMetadata(account, mailbox)
	if err != nil || meta == nil {
		return false
	}
	return time.Since(meta.LastSync) < maxAge
}
