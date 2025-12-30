package cache

import (
	"os"
	"testing"
	"time"

	"github.com/emersion/go-imap/v2"
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
