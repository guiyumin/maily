package sync

import (
	"fmt"
	"testing"
	"time"

	"github.com/emersion/go-imap/v2"

	"maily/internal/auth"
	"maily/internal/cache"
	"maily/internal/mail"
)

type fakeIMAPClient struct {
	mailboxInfo    *mail.MailboxInfo
	uidFlags       map[imap.UID]bool
	messagesByUID  map[imap.UID]mail.Email
	latestMessages []mail.Email
}

func (f *fakeIMAPClient) SelectMailboxWithInfo(string) (*mail.MailboxInfo, error) {
	return f.mailboxInfo, nil
}

func (f *fakeIMAPClient) FetchUIDsAndFlags(string, time.Time) (map[imap.UID]bool, error) {
	return f.uidFlags, nil
}

func (f *fakeIMAPClient) FetchMessagesByUIDs(_ string, uids []imap.UID) ([]mail.Email, error) {
	emails := make([]mail.Email, 0, len(uids))
	for _, uid := range uids {
		email, ok := f.messagesByUID[uid]
		if !ok {
			return nil, fmt.Errorf("missing message uid %d", uid)
		}
		emails = append(emails, email)
	}
	return emails, nil
}

func (f *fakeIMAPClient) FetchMessages(string, uint32) ([]mail.Email, error) {
	return f.latestMessages, nil
}

func (f *fakeIMAPClient) Close() error {
	return nil
}

func setTempHome(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
}

func TestFullSyncUpdatesCache(t *testing.T) {
	setTempHome(t)

	c, err := cache.New()
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	account := &auth.Account{
		Credentials: auth.Credentials{
			Email: "user@example.com",
		},
	}
	accountEmail := account.Credentials.Email
	mailbox := "INBOX"
	now := time.Now()

	if err := c.SaveEmail(accountEmail, mailbox, cache.CachedEmail{
		UID:          imap.UID(1),
		InternalDate: now.Add(-2 * time.Hour),
		Subject:      "existing-unread",
		Unread:       true,
	}); err != nil {
		t.Fatalf("SaveEmail UID 1 error: %v", err)
	}
	if err := c.SaveEmail(accountEmail, mailbox, cache.CachedEmail{
		UID:          imap.UID(2),
		InternalDate: now.Add(-3 * time.Hour),
		Subject:      "deleted",
		Unread:       true,
	}); err != nil {
		t.Fatalf("SaveEmail UID 2 error: %v", err)
	}

	fake := &fakeIMAPClient{
		mailboxInfo: &mail.MailboxInfo{UIDValidity: 123},
		uidFlags: map[imap.UID]bool{
			imap.UID(1): false,
			imap.UID(3): true,
		},
		messagesByUID: map[imap.UID]mail.Email{
			imap.UID(3): {
				UID:          imap.UID(3),
				InternalDate: now.Add(-30 * time.Minute),
				From:         "sender@example.com",
				To:           "user@example.com",
				Subject:      "new",
				Date:         now,
				Snippet:      "hello",
				Unread:       true,
			},
		},
	}

	originalFactory := newIMAPClient
	newIMAPClient = func(*auth.Credentials) (imapClient, error) {
		return fake, nil
	}
	defer func() {
		newIMAPClient = originalFactory
	}()

	syncer := NewSyncer(c, account)
	if err := syncer.FullSync(mailbox); err != nil {
		t.Fatalf("FullSync error: %v", err)
	}

	email1, err := c.GetEmail(accountEmail, mailbox, imap.UID(1))
	if err != nil {
		t.Fatalf("GetEmail UID 1 error: %v", err)
	}
	if email1 == nil || email1.Unread {
		t.Fatalf("expected UID 1 to be marked read, got %#v", email1)
	}

	email2, err := c.GetEmail(accountEmail, mailbox, imap.UID(2))
	if err != nil {
		t.Fatalf("GetEmail UID 2 error: %v", err)
	}
	if email2 != nil {
		t.Fatalf("expected UID 2 to be deleted, got %#v", email2)
	}

	email3, err := c.GetEmail(accountEmail, mailbox, imap.UID(3))
	if err != nil {
		t.Fatalf("GetEmail UID 3 error: %v", err)
	}
	if email3 == nil {
		t.Fatalf("expected UID 3 to be saved")
	}

	meta, err := c.LoadMetadata(accountEmail, mailbox)
	if err != nil {
		t.Fatalf("LoadMetadata error: %v", err)
	}
	if meta == nil || meta.UIDValidity != 123 {
		t.Fatalf("expected UIDValidity 123, got %#v", meta)
	}
}
