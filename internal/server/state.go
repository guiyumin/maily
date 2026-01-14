package server

import (
	"fmt"
	"sync"
	"time"

	"github.com/emersion/go-imap/v2"
	"maily/internal/auth"
	"maily/internal/cache"
	"maily/internal/mail"
)

const (
	// SyncDays is the number of days to look back for emails
	SyncDays = 14
	// MinSyncEmails is the minimum number of emails to sync
	MinSyncEmails = 100
)

// AccountState holds the runtime state for one account
type AccountState struct {
	Account   *auth.Account
	Syncing   bool
	LastSync  time.Time
	LastError error
	mu        sync.Mutex
}

// StateManager manages all account states and provides in-memory locking
type StateManager struct {
	accounts map[string]*AccountState // keyed by email
	store    *auth.AccountStore
	cache    *cache.Cache
	memory   *MemoryCache
	mu       sync.RWMutex
}

// NewStateManager creates a new state manager
func NewStateManager(store *auth.AccountStore, diskCache *cache.Cache) *StateManager {
	sm := &StateManager{
		accounts: make(map[string]*AccountState),
		store:    store,
		cache:    diskCache,
		memory:   NewMemoryCache(),
	}

	// Initialize state for each account
	for i := range store.Accounts {
		acc := &store.Accounts[i]
		sm.accounts[acc.Credentials.Email] = &AccountState{
			Account: acc,
		}
	}

	return sm
}

// GetAccounts returns info about all accounts
func (sm *StateManager) GetAccounts() []AccountInfo {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	var infos []AccountInfo
	for email, state := range sm.accounts {
		state.mu.Lock()
		info := AccountInfo{
			Email:      email,
			Provider:   state.Account.Credentials.Provider,
			Syncing:    state.Syncing,
			LastSync:   state.LastSync,
			EmailCount: sm.memory.Count(email, "INBOX"),
		}
		state.mu.Unlock()
		infos = append(infos, info)
	}
	return infos
}

// GetSyncStatus returns sync status for an account
func (sm *StateManager) GetSyncStatus(email string) (*SyncStatus, error) {
	sm.mu.RLock()
	state, ok := sm.accounts[email]
	sm.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("account not found: %s", email)
	}

	state.mu.Lock()
	defer state.mu.Unlock()

	status := &SyncStatus{
		Account:  email,
		Syncing:  state.Syncing,
		LastSync: state.LastSync,
	}
	if state.LastError != nil {
		status.LastError = state.LastError.Error()
	}
	return status, nil
}

// TryStartSync attempts to start a sync for an account (in-memory lock)
func (sm *StateManager) TryStartSync(email string) (bool, error) {
	sm.mu.RLock()
	state, ok := sm.accounts[email]
	sm.mu.RUnlock()

	if !ok {
		return false, fmt.Errorf("account not found: %s", email)
	}

	state.mu.Lock()
	defer state.mu.Unlock()

	if state.Syncing {
		return false, nil // Already syncing
	}

	state.Syncing = true
	return true, nil
}

// EndSync marks sync as complete for an account
func (sm *StateManager) EndSync(email string, err error) {
	sm.mu.RLock()
	state, ok := sm.accounts[email]
	sm.mu.RUnlock()

	if !ok {
		return
	}

	state.mu.Lock()
	defer state.mu.Unlock()

	state.Syncing = false
	state.LastSync = time.Now()
	state.LastError = err
}

// GetEmails returns emails from memory cache, falling back to disk
func (sm *StateManager) GetEmails(email, mailbox string, limit int) ([]cache.CachedEmail, error) {
	// Try memory first
	emails := sm.memory.Get(email, mailbox)
	if len(emails) > 0 {
		if limit > 0 && len(emails) > limit {
			return emails[:limit], nil
		}
		return emails, nil
	}

	// Fall back to disk cache
	if sm.cache != nil {
		diskEmails, err := sm.cache.LoadEmails(email, mailbox)
		if err != nil {
			return nil, err
		}
		// Store in memory for next time
		sm.memory.Set(email, mailbox, diskEmails)
		if limit > 0 && len(diskEmails) > limit {
			return diskEmails[:limit], nil
		}
		return diskEmails, nil
	}

	return nil, nil
}

// GetEmail returns a single email by UID
func (sm *StateManager) GetEmail(email, mailbox string, uid imap.UID) (*cache.CachedEmail, error) {
	// Try memory first
	cached := sm.memory.GetByUID(email, mailbox, uid)
	if cached != nil {
		return cached, nil
	}

	// Fall back to disk
	if sm.cache != nil {
		return sm.cache.GetEmail(email, mailbox, uid)
	}

	return nil, nil
}

// UpdateEmail updates an email in both memory and disk cache
func (sm *StateManager) UpdateEmail(email, mailbox string, cached cache.CachedEmail) error {
	sm.memory.Update(email, mailbox, cached)
	if sm.cache != nil {
		return sm.cache.SaveEmail(email, mailbox, cached)
	}
	return nil
}

// DeleteEmail removes an email from both memory and disk cache
func (sm *StateManager) DeleteEmail(email, mailbox string, uid imap.UID) error {
	sm.memory.Delete(email, mailbox, uid)
	if sm.cache != nil {
		return sm.cache.DeleteEmail(email, mailbox, uid)
	}
	return nil
}

// SetEmails stores emails in memory cache
func (sm *StateManager) SetEmails(email, mailbox string, emails []cache.CachedEmail) {
	sm.memory.Set(email, mailbox, emails)
}

// GetAccountCredentials returns credentials for an account
func (sm *StateManager) GetAccountCredentials(email string) (*auth.Credentials, error) {
	sm.mu.RLock()
	state, ok := sm.accounts[email]
	sm.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("account not found: %s", email)
	}

	return &state.Account.Credentials, nil
}

// GetLabels fetches labels from IMAP
func (sm *StateManager) GetLabels(email string) ([]string, error) {
	creds, err := sm.GetAccountCredentials(email)
	if err != nil {
		return nil, err
	}

	client, err := mail.NewIMAPClient(creds)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	return client.ListMailboxes()
}

// Sync performs a full sync for an account using max(14 days, 100 emails)
// This ensures we always have at least 100 emails while never missing recent ones
func (sm *StateManager) Sync(email, mailbox string) error {
	acquired, err := sm.TryStartSync(email)
	if err != nil {
		return err
	}
	if !acquired {
		return fmt.Errorf("sync already in progress")
	}
	defer sm.EndSync(email, nil)

	creds, err := sm.GetAccountCredentials(email)
	if err != nil {
		sm.EndSync(email, err)
		return err
	}

	client, err := mail.NewIMAPClient(creds)
	if err != nil {
		sm.EndSync(email, err)
		return err
	}
	defer client.Close()

	// Step 1: Fetch last 100 emails by sequence number
	emails, err := client.FetchMessages(mailbox, MinSyncEmails)
	if err != nil {
		sm.EndSync(email, err)
		return err
	}

	// Build map of fetched UIDs
	fetchedUIDs := make(map[imap.UID]bool)
	for _, e := range emails {
		fetchedUIDs[e.UID] = true
	}

	// Step 2: Get UIDs from last 14 days
	since := time.Now().AddDate(0, 0, -SyncDays)
	recentUIDs, err := client.FetchUIDsAndFlags(mailbox, since)
	if err != nil {
		// Non-fatal: we still have the 100 emails
		recentUIDs = nil
	}

	// Step 3: Find UIDs in 14-day window not already fetched
	var missingUIDs []imap.UID
	for uid := range recentUIDs {
		if !fetchedUIDs[uid] {
			missingUIDs = append(missingUIDs, uid)
		}
	}

	// Step 4: Fetch any missing recent emails
	if len(missingUIDs) > 0 {
		additional, err := client.FetchMessagesByUIDs(mailbox, missingUIDs)
		if err == nil {
			emails = append(emails, additional...)
		}
	}

	// Convert to cached format and store
	cached := make([]cache.CachedEmail, len(emails))
	for i, e := range emails {
		cached[i] = emailToCached(e)
	}

	sm.SetEmails(email, mailbox, cached)

	// Also persist to disk
	if sm.cache != nil {
		for _, c := range cached {
			sm.cache.SaveEmail(email, mailbox, c)
		}
	}

	return nil
}

// emailToCached converts mail.Email to cache.CachedEmail
func emailToCached(e mail.Email) cache.CachedEmail {
	attachments := make([]cache.Attachment, len(e.Attachments))
	for i, a := range e.Attachments {
		attachments[i] = cache.Attachment{
			PartID:      a.PartID,
			Filename:    a.Filename,
			ContentType: a.ContentType,
			Size:        a.Size,
			Encoding:    a.Encoding,
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
		BodyHTML:     e.BodyHTML,
		Unread:       e.Unread,
		References:   e.References,
		Attachments:  attachments,
	}
}

// ProcessPendingOps processes all pending operations from the queue
// Returns the number of successfully processed operations
func (sm *StateManager) ProcessPendingOps() (processed int, failed int) {
	if sm.cache == nil {
		return 0, 0
	}

	ops, err := sm.cache.GetPendingOps("")
	if err != nil || len(ops) == 0 {
		return 0, 0
	}

	// Group ops by account to reuse IMAP connections
	byAccount := make(map[string][]cache.PendingOp)
	for _, op := range ops {
		byAccount[op.Account] = append(byAccount[op.Account], op)
	}

	for account, accountOps := range byAccount {
		creds, err := sm.GetAccountCredentials(account)
		if err != nil {
			// Mark all ops for this account as failed
			for _, op := range accountOps {
				sm.cache.UpdatePendingOpError(op.ID, err.Error())
				failed++
			}
			continue
		}

		client, err := mail.NewIMAPClient(creds)
		if err != nil {
			for _, op := range accountOps {
				sm.cache.UpdatePendingOpError(op.ID, err.Error())
				failed++
			}
			continue
		}

		for _, op := range accountOps {
			var opErr error
			switch op.Operation {
			case cache.OpDelete:
				opErr = client.DeleteMessage(op.UID)
			case cache.OpMoveTrash:
				opErr = client.MoveToTrashFromMailbox([]imap.UID{op.UID}, op.Mailbox)
			case cache.OpMarkRead:
				opErr = client.MarkAsRead(op.UID)
			default:
				opErr = fmt.Errorf("unknown operation: %s", op.Operation)
			}

			if opErr != nil {
				sm.cache.UpdatePendingOpError(op.ID, opErr.Error())
				sm.cache.LogOp(op, cache.StatusFailed, opErr.Error())
				failed++
			} else {
				sm.cache.RemovePendingOp(op.ID)
				sm.cache.LogOp(op, cache.StatusSuccess, "")
				// Delete from cache again in case sync pulled email back
				if op.Operation == cache.OpDelete || op.Operation == cache.OpMoveTrash {
					sm.cache.DeleteEmail(op.Account, op.Mailbox, op.UID)
					sm.memory.Delete(op.Account, op.Mailbox, op.UID)
				}
				processed++
			}
		}

		client.Close()
	}

	return processed, failed
}

// GetPendingOpsCount returns the number of pending operations
func (sm *StateManager) GetPendingOpsCount() int {
	if sm.cache == nil {
		return 0
	}
	count, _ := sm.cache.GetPendingOpsCount()
	return count
}
