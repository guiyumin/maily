package cache

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/emersion/go-imap/v2"
	_ "github.com/mattn/go-sqlite3"

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
	BodyHTML     string       `json:"body_html"`
	Unread       bool         `json:"unread"`
	References   string       `json:"references,omitempty"`
	Attachments  []Attachment `json:"attachments,omitempty"`
}

// Metadata tracks mailbox sync state
type Metadata struct {
	UIDValidity uint32    `json:"uidvalidity"`
	LastSync    time.Time `json:"last_sync"`
}

// Operation types for pending operations
const (
	OpDelete    = "delete"
	OpMoveTrash = "move_trash"
	OpMarkRead  = "mark_read"
)

// PendingOp represents a pending email operation to be synced
type PendingOp struct {
	ID        int64
	Account   string
	Mailbox   string
	Operation string
	UID       imap.UID
	CreatedAt time.Time
	Retries   int
	LastError string
}

// Cache manages persistent email storage using SQLite
type Cache struct {
	db     *sql.DB
	dbPath string
}

const schema = `
CREATE TABLE IF NOT EXISTS mailbox_metadata (
    account TEXT NOT NULL,
    mailbox TEXT NOT NULL,
    uid_validity INTEGER NOT NULL DEFAULT 0,
    last_sync INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (account, mailbox)
);

CREATE TABLE IF NOT EXISTS emails (
    account TEXT NOT NULL,
    mailbox TEXT NOT NULL,
    uid INTEGER NOT NULL,
    message_id TEXT NOT NULL DEFAULT '',
    internal_date INTEGER NOT NULL,
    from_addr TEXT NOT NULL DEFAULT '',
    reply_to TEXT NOT NULL DEFAULT '',
    to_addr TEXT NOT NULL DEFAULT '',
    subject TEXT NOT NULL DEFAULT '',
    date INTEGER NOT NULL DEFAULT 0,
    snippet TEXT NOT NULL DEFAULT '',
    body_html TEXT NOT NULL DEFAULT '',
    unread INTEGER NOT NULL DEFAULT 1,
    references_hdr TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (account, mailbox, uid)
);

CREATE TABLE IF NOT EXISTS attachments (
    account TEXT NOT NULL,
    mailbox TEXT NOT NULL,
    email_uid INTEGER NOT NULL,
    part_id TEXT NOT NULL,
    filename TEXT NOT NULL DEFAULT '',
    content_type TEXT NOT NULL DEFAULT '',
    size INTEGER NOT NULL DEFAULT 0,
    encoding TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (account, mailbox, email_uid, part_id),
    FOREIGN KEY (account, mailbox, email_uid)
        REFERENCES emails(account, mailbox, uid) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS sync_locks (
    account TEXT PRIMARY KEY,
    pid INTEGER NOT NULL,
    start_time TEXT NOT NULL DEFAULT '',
    locked_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS pending_ops (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    account TEXT NOT NULL,
    mailbox TEXT NOT NULL,
    operation TEXT NOT NULL,
    uid INTEGER NOT NULL,
    created_at INTEGER NOT NULL,
    retries INTEGER NOT NULL DEFAULT 0,
    last_error TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_emails_date ON emails(account, mailbox, internal_date DESC);
CREATE INDEX IF NOT EXISTS idx_emails_internal_date ON emails(internal_date);
CREATE INDEX IF NOT EXISTS idx_pending_ops_account ON pending_ops(account);
`

// New creates a new cache instance with SQLite backend
func New() (*Cache, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	dbDir := filepath.Join(homeDir, ".config", "maily")
	if err := os.MkdirAll(dbDir, 0700); err != nil {
		return nil, err
	}

	dbPath := filepath.Join(dbDir, "maily.db")
	return openDB(dbPath)
}

// NewWithPath creates a cache with a custom database path (for testing)
func NewWithPath(dbPath string) (*Cache, error) {
	return openDB(dbPath)
}

func openDB(dbPath string) (*Cache, error) {
	db, err := sql.Open("sqlite3", dbPath+"?_journal=WAL&_timeout=5000&_fk=1")
	if err != nil {
		return nil, err
	}

	// Create schema
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create schema: %w", err)
	}

	c := &Cache{db: db, dbPath: dbPath}

	// Clean up old JSON cache directory if it exists
	c.cleanupOldCache()

	return c, nil
}

// cleanupOldCache removes the old JSON file-based cache directory
func (c *Cache) cleanupOldCache() {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return
	}
	oldCacheDir := filepath.Join(homeDir, ".config", "maily", "cache")
	if _, err := os.Stat(oldCacheDir); err == nil {
		os.RemoveAll(oldCacheDir)
	}
}

// Close closes the database connection
func (c *Cache) Close() error {
	if c.db != nil {
		return c.db.Close()
	}
	return nil
}

// AcquireLock tries to acquire the sync lock for an account
// Returns true if lock acquired, false if already locked
func (c *Cache) AcquireLock(account string) (bool, error) {
	// Check if lock exists and is active
	var pid int
	var startTime string
	var lockedAt int64
	err := c.db.QueryRow(
		"SELECT pid, start_time, locked_at FROM sync_locks WHERE account = ?",
		account,
	).Scan(&pid, &startTime, &lockedAt)

	if err == nil {
		// Lock exists, check if it's still active
		info := proc.LockInfo{PID: pid, Start: startTime}
		if lockActive(info) {
			return false, nil
		}
		// Stale lock, remove it
		c.db.Exec("DELETE FROM sync_locks WHERE account = ?", account)
	}

	// Try to acquire lock
	currentPID := os.Getpid()
	currentStart, _ := proc.StartTime(currentPID)

	_, err = c.db.Exec(
		"INSERT OR REPLACE INTO sync_locks (account, pid, start_time, locked_at) VALUES (?, ?, ?, ?)",
		account, currentPID, currentStart, time.Now().Unix(),
	)
	if err != nil {
		return false, err
	}

	return true, nil
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

// ReleaseLock releases the sync lock for an account
func (c *Cache) ReleaseLock(account string) error {
	_, err := c.db.Exec("DELETE FROM sync_locks WHERE account = ?", account)
	return err
}

// GetSyncLocks returns all active sync locks (for update.go)
func (c *Cache) GetSyncLocks() ([]proc.LockInfo, error) {
	rows, err := c.db.Query("SELECT account, pid, start_time FROM sync_locks")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var locks []proc.LockInfo
	for rows.Next() {
		var account string
		var info proc.LockInfo
		if err := rows.Scan(&account, &info.PID, &info.Start); err != nil {
			continue
		}
		locks = append(locks, info)
	}
	return locks, nil
}

// CleanupStaleLocks removes locks for dead processes
func (c *Cache) CleanupStaleLocks() {
	locks, err := c.GetSyncLocks()
	if err != nil {
		return
	}

	for _, info := range locks {
		if !lockActive(info) {
			c.db.Exec("DELETE FROM sync_locks WHERE pid = ?", info.PID)
		}
	}
}

// LoadMetadata loads mailbox metadata
func (c *Cache) LoadMetadata(account, mailbox string) (*Metadata, error) {
	var uidValidity uint32
	var lastSync int64

	err := c.db.QueryRow(
		"SELECT uid_validity, last_sync FROM mailbox_metadata WHERE account = ? AND mailbox = ?",
		account, mailbox,
	).Scan(&uidValidity, &lastSync)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &Metadata{
		UIDValidity: uidValidity,
		LastSync:    time.Unix(lastSync, 0),
	}, nil
}

// SaveMetadata saves mailbox metadata
func (c *Cache) SaveMetadata(account, mailbox string, meta *Metadata) error {
	_, err := c.db.Exec(
		"INSERT OR REPLACE INTO mailbox_metadata (account, mailbox, uid_validity, last_sync) VALUES (?, ?, ?, ?)",
		account, mailbox, meta.UIDValidity, meta.LastSync.Unix(),
	)
	return err
}

// LoadEmails loads all cached emails for a mailbox, sorted by InternalDate descending
func (c *Cache) LoadEmails(account, mailbox string) ([]CachedEmail, error) {
	rows, err := c.db.Query(`
		SELECT uid, message_id, internal_date, from_addr, reply_to, to_addr,
		       subject, date, snippet, body_html, unread, references_hdr
		FROM emails
		WHERE account = ? AND mailbox = ?
		ORDER BY internal_date DESC
	`, account, mailbox)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var emails []CachedEmail
	for rows.Next() {
		var email CachedEmail
		var uid uint32
		var internalDate, date int64
		var unread int

		err := rows.Scan(
			&uid, &email.MessageID, &internalDate, &email.From, &email.ReplyTo,
			&email.To, &email.Subject, &date, &email.Snippet, &email.BodyHTML,
			&unread, &email.References,
		)
		if err != nil {
			continue
		}

		email.UID = imap.UID(uid)
		email.InternalDate = time.Unix(internalDate, 0)
		email.Date = time.Unix(date, 0)
		email.Unread = unread == 1

		// Load attachments
		email.Attachments, _ = c.loadAttachments(account, mailbox, uid)

		emails = append(emails, email)
	}

	return emails, nil
}

// loadAttachments loads attachments for an email
func (c *Cache) loadAttachments(account, mailbox string, uid uint32) ([]Attachment, error) {
	rows, err := c.db.Query(`
		SELECT part_id, filename, content_type, size, encoding
		FROM attachments
		WHERE account = ? AND mailbox = ? AND email_uid = ?
	`, account, mailbox, uid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var attachments []Attachment
	for rows.Next() {
		var att Attachment
		if err := rows.Scan(&att.PartID, &att.Filename, &att.ContentType, &att.Size, &att.Encoding); err != nil {
			continue
		}
		attachments = append(attachments, att)
	}
	return attachments, nil
}

// LoadEmailsLimit loads up to limit emails, sorted by InternalDate descending
func (c *Cache) LoadEmailsLimit(account, mailbox string, limit int) ([]CachedEmail, error) {
	rows, err := c.db.Query(`
		SELECT uid, message_id, internal_date, from_addr, reply_to, to_addr,
		       subject, date, snippet, body_html, unread, references_hdr
		FROM emails
		WHERE account = ? AND mailbox = ?
		ORDER BY internal_date DESC
		LIMIT ?
	`, account, mailbox, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var emails []CachedEmail
	for rows.Next() {
		var email CachedEmail
		var uid uint32
		var internalDate, date int64
		var unread int

		err := rows.Scan(
			&uid, &email.MessageID, &internalDate, &email.From, &email.ReplyTo,
			&email.To, &email.Subject, &date, &email.Snippet, &email.BodyHTML,
			&unread, &email.References,
		)
		if err != nil {
			continue
		}

		email.UID = imap.UID(uid)
		email.InternalDate = time.Unix(internalDate, 0)
		email.Date = time.Unix(date, 0)
		email.Unread = unread == 1

		// Load attachments
		email.Attachments, _ = c.loadAttachments(account, mailbox, uid)

		emails = append(emails, email)
	}

	return emails, nil
}

// SaveEmail saves a single email to cache
func (c *Cache) SaveEmail(account, mailbox string, email CachedEmail) error {
	tx, err := c.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	unread := 0
	if email.Unread {
		unread = 1
	}

	_, err = tx.Exec(`
		INSERT OR REPLACE INTO emails
		(account, mailbox, uid, message_id, internal_date, from_addr, reply_to,
		 to_addr, subject, date, snippet, body_html, unread, references_hdr)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		account, mailbox, uint32(email.UID), email.MessageID,
		email.InternalDate.Unix(), email.From, email.ReplyTo, email.To,
		email.Subject, email.Date.Unix(), email.Snippet, email.BodyHTML,
		unread, email.References,
	)
	if err != nil {
		return err
	}

	// Delete existing attachments and re-insert
	tx.Exec("DELETE FROM attachments WHERE account = ? AND mailbox = ? AND email_uid = ?",
		account, mailbox, uint32(email.UID))

	for _, att := range email.Attachments {
		_, err = tx.Exec(`
			INSERT INTO attachments
			(account, mailbox, email_uid, part_id, filename, content_type, size, encoding)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		`,
			account, mailbox, uint32(email.UID), att.PartID, att.Filename,
			att.ContentType, att.Size, att.Encoding,
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// DeleteEmail deletes an email from cache
func (c *Cache) DeleteEmail(account, mailbox string, uid imap.UID) error {
	_, err := c.db.Exec(
		"DELETE FROM emails WHERE account = ? AND mailbox = ? AND uid = ?",
		account, mailbox, uint32(uid),
	)
	return err
}

// GetCachedUIDs returns a set of all cached UIDs for a mailbox
func (c *Cache) GetCachedUIDs(account, mailbox string) (map[imap.UID]bool, error) {
	rows, err := c.db.Query(
		"SELECT uid FROM emails WHERE account = ? AND mailbox = ?",
		account, mailbox,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	uids := make(map[imap.UID]bool)
	for rows.Next() {
		var uid uint32
		if err := rows.Scan(&uid); err != nil {
			continue
		}
		uids[imap.UID(uid)] = true
	}

	return uids, nil
}

// Cleanup deletes cached emails older than the given time
func (c *Cache) Cleanup(account, mailbox string, olderThan time.Time) (int, error) {
	result, err := c.db.Exec(
		"DELETE FROM emails WHERE account = ? AND mailbox = ? AND internal_date < ?",
		account, mailbox, olderThan.Unix(),
	)
	if err != nil {
		return 0, err
	}

	affected, _ := result.RowsAffected()
	return int(affected), nil
}

// InvalidateMailbox removes all cached emails for a mailbox
func (c *Cache) InvalidateMailbox(account, mailbox string) error {
	_, err := c.db.Exec(
		"DELETE FROM emails WHERE account = ? AND mailbox = ?",
		account, mailbox,
	)
	return err
}

// GetEmail loads a single email by UID
func (c *Cache) GetEmail(account, mailbox string, uid imap.UID) (*CachedEmail, error) {
	var email CachedEmail
	var uidVal uint32
	var internalDate, date int64
	var unread int

	err := c.db.QueryRow(`
		SELECT uid, message_id, internal_date, from_addr, reply_to, to_addr,
		       subject, date, snippet, body_html, unread, references_hdr
		FROM emails
		WHERE account = ? AND mailbox = ? AND uid = ?
	`, account, mailbox, uint32(uid)).Scan(
		&uidVal, &email.MessageID, &internalDate, &email.From, &email.ReplyTo,
		&email.To, &email.Subject, &date, &email.Snippet, &email.BodyHTML,
		&unread, &email.References,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	email.UID = imap.UID(uidVal)
	email.InternalDate = time.Unix(internalDate, 0)
	email.Date = time.Unix(date, 0)
	email.Unread = unread == 1

	// Load attachments
	email.Attachments, _ = c.loadAttachments(account, mailbox, uidVal)

	return &email, nil
}

// UpdateEmailFlags updates only the Unread flag of a cached email
func (c *Cache) UpdateEmailFlags(account, mailbox string, uid imap.UID, unread bool) error {
	unreadVal := 0
	if unread {
		unreadVal = 1
	}

	_, err := c.db.Exec(
		"UPDATE emails SET unread = ? WHERE account = ? AND mailbox = ? AND uid = ?",
		unreadVal, account, mailbox, uint32(uid),
	)
	return err
}

// IsFresh returns true if the cache was synced within the given duration
func (c *Cache) IsFresh(account, mailbox string, maxAge time.Duration) bool {
	meta, err := c.LoadMetadata(account, mailbox)
	if err != nil || meta == nil {
		return false
	}
	return time.Since(meta.LastSync) < maxAge
}

// MarshalJSON for Attachment (for compatibility)
func (a Attachment) MarshalJSON() ([]byte, error) {
	type Alias Attachment
	return json.Marshal(Alias(a))
}

// MarshalJSON for CachedEmail (for compatibility)
func (e CachedEmail) MarshalJSON() ([]byte, error) {
	type Alias CachedEmail
	return json.Marshal(Alias(e))
}

// AddPendingOp adds a pending operation to the queue
func (c *Cache) AddPendingOp(account, mailbox, operation string, uid imap.UID) error {
	_, err := c.db.Exec(`
		INSERT INTO pending_ops (account, mailbox, operation, uid, created_at)
		VALUES (?, ?, ?, ?, ?)
	`, account, mailbox, operation, uint32(uid), time.Now().Unix())
	return err
}

// GetPendingOps returns all pending operations, optionally filtered by account
func (c *Cache) GetPendingOps(account string) ([]PendingOp, error) {
	var rows *sql.Rows
	var err error

	if account == "" {
		rows, err = c.db.Query(`
			SELECT id, account, mailbox, operation, uid, created_at, retries, last_error
			FROM pending_ops ORDER BY created_at ASC
		`)
	} else {
		rows, err = c.db.Query(`
			SELECT id, account, mailbox, operation, uid, created_at, retries, last_error
			FROM pending_ops WHERE account = ? ORDER BY created_at ASC
		`, account)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ops []PendingOp
	for rows.Next() {
		var op PendingOp
		var uid uint32
		var createdAt int64
		if err := rows.Scan(&op.ID, &op.Account, &op.Mailbox, &op.Operation,
			&uid, &createdAt, &op.Retries, &op.LastError); err != nil {
			continue
		}
		op.UID = imap.UID(uid)
		op.CreatedAt = time.Unix(createdAt, 0)
		ops = append(ops, op)
	}
	return ops, nil
}

// RemovePendingOp removes a pending operation by ID
func (c *Cache) RemovePendingOp(id int64) error {
	_, err := c.db.Exec("DELETE FROM pending_ops WHERE id = ?", id)
	return err
}

// UpdatePendingOpError updates the retry count and last error for a pending operation
func (c *Cache) UpdatePendingOpError(id int64, errMsg string) error {
	_, err := c.db.Exec(`
		UPDATE pending_ops SET retries = retries + 1, last_error = ? WHERE id = ?
	`, errMsg, id)
	return err
}

// GetPendingOpsCount returns the count of pending operations
func (c *Cache) GetPendingOpsCount() (int, error) {
	var count int
	err := c.db.QueryRow("SELECT COUNT(*) FROM pending_ops").Scan(&count)
	return count, err
}
