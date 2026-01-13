package cache

import (
	"os"
	"testing"
	"time"

	"github.com/emersion/go-imap/v2"

	"maily/internal/proc"
)

func setTempHome(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
}

func TestCacheSaveLoadAndFlags(t *testing.T) {
	setTempHome(t)

	c, err := New()
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer c.Close()

	account := "user@example.com"
	mailbox := "INBOX"
	now := time.Now()

	older := CachedEmail{
		UID:          imap.UID(1),
		InternalDate: now.Add(-2 * time.Hour),
		Subject:      "old",
		Unread:       true,
	}
	newer := CachedEmail{
		UID:          imap.UID(2),
		InternalDate: now.Add(-1 * time.Hour),
		Subject:      "new",
		Unread:       true,
	}

	if err := c.SaveEmail(account, mailbox, older); err != nil {
		t.Fatalf("SaveEmail older error: %v", err)
	}
	if err := c.SaveEmail(account, mailbox, newer); err != nil {
		t.Fatalf("SaveEmail newer error: %v", err)
	}

	emails, err := c.LoadEmails(account, mailbox)
	if err != nil {
		t.Fatalf("LoadEmails error: %v", err)
	}
	if len(emails) != 2 {
		t.Fatalf("expected 2 emails, got %d", len(emails))
	}
	if emails[0].UID != newer.UID || emails[1].UID != older.UID {
		t.Fatalf("emails not sorted newest-first: %+v", []imap.UID{emails[0].UID, emails[1].UID})
	}

	if err := c.UpdateEmailFlags(account, mailbox, newer.UID, false); err != nil {
		t.Fatalf("UpdateEmailFlags error: %v", err)
	}

	updated, err := c.GetEmail(account, mailbox, newer.UID)
	if err != nil {
		t.Fatalf("GetEmail error: %v", err)
	}
	if updated == nil || updated.Unread {
		t.Fatalf("expected updated email to be read, got %#v", updated)
	}

	uids, err := c.GetCachedUIDs(account, mailbox)
	if err != nil {
		t.Fatalf("GetCachedUIDs error: %v", err)
	}
	if !uids[older.UID] || !uids[newer.UID] {
		t.Fatalf("expected cached UIDs to include %d and %d", older.UID, newer.UID)
	}
}

func TestCacheCleanup(t *testing.T) {
	setTempHome(t)

	c, err := New()
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer c.Close()

	account := "user@example.com"
	mailbox := "INBOX"
	now := time.Now()

	oldEmail := CachedEmail{
		UID:          imap.UID(10),
		InternalDate: now.Add(-48 * time.Hour),
		Subject:      "old",
	}
	newEmail := CachedEmail{
		UID:          imap.UID(11),
		InternalDate: now.Add(-1 * time.Hour),
		Subject:      "new",
	}

	if err := c.SaveEmail(account, mailbox, oldEmail); err != nil {
		t.Fatalf("SaveEmail old error: %v", err)
	}
	if err := c.SaveEmail(account, mailbox, newEmail); err != nil {
		t.Fatalf("SaveEmail new error: %v", err)
	}

	deleted, err := c.Cleanup(account, mailbox, now.Add(-24*time.Hour))
	if err != nil {
		t.Fatalf("Cleanup error: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("expected 1 deleted email, got %d", deleted)
	}

	oldResult, err := c.GetEmail(account, mailbox, oldEmail.UID)
	if err != nil {
		t.Fatalf("GetEmail old error: %v", err)
	}
	if oldResult != nil {
		t.Fatalf("expected old email to be removed, got %#v", oldResult)
	}

	newResult, err := c.GetEmail(account, mailbox, newEmail.UID)
	if err != nil {
		t.Fatalf("GetEmail new error: %v", err)
	}
	if newResult == nil {
		t.Fatalf("expected new email to remain")
	}
}

func TestCacheAcquireLock(t *testing.T) {
	setTempHome(t)

	c, err := New()
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer c.Close()

	account := "user@example.com"

	acquired, err := c.AcquireLock(account)
	if err != nil {
		t.Fatalf("AcquireLock error: %v", err)
	}
	if !acquired {
		t.Fatalf("expected lock to be acquired")
	}

	acquired, err = c.AcquireLock(account)
	if err != nil {
		t.Fatalf("AcquireLock second error: %v", err)
	}
	if acquired {
		t.Fatalf("expected lock to be held by current process")
	}

	if err := c.ReleaseLock(account); err != nil && !os.IsNotExist(err) {
		t.Fatalf("ReleaseLock error: %v", err)
	}

	acquired, err = c.AcquireLock(account)
	if err != nil {
		t.Fatalf("AcquireLock after release error: %v", err)
	}
	if !acquired {
		t.Fatalf("expected lock to be acquired after release")
	}

	if err := c.ReleaseLock(account); err != nil && !os.IsNotExist(err) {
		t.Fatalf("ReleaseLock final error: %v", err)
	}
}

func TestCacheAcquireLockMismatchedStart(t *testing.T) {
	setTempHome(t)

	c, err := New()
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer c.Close()

	account := "user@example.com"

	// Insert a lock with mismatched start time directly into DB
	_, err = c.db.Exec(
		"INSERT INTO sync_locks (account, pid, start_time, locked_at) VALUES (?, ?, ?, ?)",
		account, os.Getpid(), "bogus", time.Now().Unix(),
	)
	if err != nil {
		t.Fatalf("Insert lock error: %v", err)
	}

	originalStartTime := proc.StartTime
	proc.StartTime = func(int) (string, error) {
		return "actual", nil
	}
	defer func() {
		proc.StartTime = originalStartTime
	}()

	acquired, err := c.AcquireLock(account)
	if err != nil {
		t.Fatalf("AcquireLock error: %v", err)
	}
	if !acquired {
		t.Fatalf("expected lock to be acquired after mismatched start")
	}

	if err := c.ReleaseLock(account); err != nil && !os.IsNotExist(err) {
		t.Fatalf("ReleaseLock error: %v", err)
	}
}

func TestCacheAttachments(t *testing.T) {
	setTempHome(t)

	c, err := New()
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer c.Close()

	account := "user@example.com"
	mailbox := "INBOX"

	email := CachedEmail{
		UID:          imap.UID(100),
		InternalDate: time.Now(),
		Subject:      "with attachments",
		Attachments: []Attachment{
			{PartID: "1", Filename: "doc.pdf", ContentType: "application/pdf", Size: 1024},
			{PartID: "2", Filename: "image.png", ContentType: "image/png", Size: 2048},
		},
	}

	if err := c.SaveEmail(account, mailbox, email); err != nil {
		t.Fatalf("SaveEmail error: %v", err)
	}

	loaded, err := c.GetEmail(account, mailbox, email.UID)
	if err != nil {
		t.Fatalf("GetEmail error: %v", err)
	}
	if loaded == nil {
		t.Fatalf("expected email to be loaded")
	}
	if len(loaded.Attachments) != 2 {
		t.Fatalf("expected 2 attachments, got %d", len(loaded.Attachments))
	}
	if loaded.Attachments[0].Filename != "doc.pdf" {
		t.Fatalf("expected first attachment to be doc.pdf, got %s", loaded.Attachments[0].Filename)
	}
}

func TestCacheMetadata(t *testing.T) {
	setTempHome(t)

	c, err := New()
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer c.Close()

	account := "user@example.com"
	mailbox := "INBOX"

	meta := &Metadata{
		UIDValidity: 12345,
		LastSync:    time.Now().Truncate(time.Second),
	}

	if err := c.SaveMetadata(account, mailbox, meta); err != nil {
		t.Fatalf("SaveMetadata error: %v", err)
	}

	loaded, err := c.LoadMetadata(account, mailbox)
	if err != nil {
		t.Fatalf("LoadMetadata error: %v", err)
	}
	if loaded == nil {
		t.Fatalf("expected metadata to be loaded")
	}
	if loaded.UIDValidity != meta.UIDValidity {
		t.Fatalf("expected UIDValidity %d, got %d", meta.UIDValidity, loaded.UIDValidity)
	}
	if !loaded.LastSync.Equal(meta.LastSync) {
		t.Fatalf("expected LastSync %v, got %v", meta.LastSync, loaded.LastSync)
	}
}

func TestCacheIsFresh(t *testing.T) {
	setTempHome(t)

	c, err := New()
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer c.Close()

	account := "user@example.com"
	mailbox := "INBOX"

	// No metadata - not fresh
	if c.IsFresh(account, mailbox, time.Hour) {
		t.Fatalf("expected not fresh without metadata")
	}

	// Fresh metadata
	meta := &Metadata{
		UIDValidity: 1,
		LastSync:    time.Now(),
	}
	if err := c.SaveMetadata(account, mailbox, meta); err != nil {
		t.Fatalf("SaveMetadata error: %v", err)
	}

	if !c.IsFresh(account, mailbox, time.Hour) {
		t.Fatalf("expected fresh with recent metadata")
	}

	// Stale metadata
	meta.LastSync = time.Now().Add(-2 * time.Hour)
	if err := c.SaveMetadata(account, mailbox, meta); err != nil {
		t.Fatalf("SaveMetadata error: %v", err)
	}

	if c.IsFresh(account, mailbox, time.Hour) {
		t.Fatalf("expected not fresh with old metadata")
	}
}
