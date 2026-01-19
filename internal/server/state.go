package server

import (
	"errors"
	"fmt"
	"strings"
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
	imapMu    sync.Mutex
	imapClient *mail.IMAPClient
}

// StateManager manages all account states and IMAP connections
type StateManager struct {
	accounts map[string]*AccountState // keyed by email
	store    *auth.AccountStore
	cache    *cache.Cache // SQLite disk cache - single source of truth
	mu       sync.RWMutex
}

// NewStateManager creates a new state manager
func NewStateManager(store *auth.AccountStore, diskCache *cache.Cache) *StateManager {
	sm := &StateManager{
		accounts: make(map[string]*AccountState),
		store:    store,
		cache:    diskCache,
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

func (sm *StateManager) getAccountState(email string) (*AccountState, error) {
	sm.mu.RLock()
	state, ok := sm.accounts[email]
	sm.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("account not found: %s", email)
	}
	return state, nil
}

func isConnectionError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "closed network connection") ||
		strings.Contains(errStr, "connection reset") ||
		strings.Contains(errStr, "broken pipe") ||
		strings.Contains(errStr, "EOF")
}

func (sm *StateManager) ensureIMAPClientLocked(state *AccountState) (*mail.IMAPClient, error) {
	if state.imapClient != nil {
		return state.imapClient, nil
	}
	client, err := mail.NewIMAPClient(&state.Account.Credentials)
	if err != nil {
		return nil, err
	}
	state.imapClient = client
	return client, nil
}

func (sm *StateManager) withIMAPClient(email string, fn func(*mail.IMAPClient) error) error {
	state, err := sm.getAccountState(email)
	if err != nil {
		return err
	}

	state.imapMu.Lock()
	defer state.imapMu.Unlock()

	client, err := sm.ensureIMAPClientLocked(state)
	if err != nil {
		return err
	}

	err = fn(client)
	if isConnectionError(err) && state.imapClient != nil {
		state.imapClient.Close()
		state.imapClient = nil
	}
	return err
}

func (sm *StateManager) CloseIMAPClients() {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	for _, state := range sm.accounts {
		state.imapMu.Lock()
		if state.imapClient != nil {
			state.imapClient.Close()
			state.imapClient = nil
		}
		state.imapMu.Unlock()
	}
}

// IsCacheFresh reports whether the disk cache was synced recently.
func (sm *StateManager) IsCacheFresh(email, mailbox string, maxAge time.Duration) bool {
	if sm.cache == nil {
		return false
	}
	return sm.cache.IsFresh(email, mailbox, maxAge)
}

// GetAccounts returns info about all accounts
func (sm *StateManager) GetAccounts() []AccountInfo {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	var infos []AccountInfo
	for email, state := range sm.accounts {
		state.mu.Lock()
		emailCount := 0
		if sm.cache != nil {
			emailCount, _ = sm.cache.CountEmails(email, "INBOX")
		}
		info := AccountInfo{
			Email:      email,
			Provider:   state.Account.Credentials.Provider,
			Syncing:    state.Syncing,
			LastSync:   state.LastSync,
			EmailCount: emailCount,
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

// GetEmails returns emails from disk cache
func (sm *StateManager) GetEmails(email, mailbox string, limit int) ([]cache.CachedEmail, error) {
	if sm.cache == nil {
		return nil, nil
	}
	emails, err := sm.cache.LoadEmails(email, mailbox)
	if err != nil {
		return nil, err
	}
	if limit > 0 && len(emails) > limit {
		return emails[:limit], nil
	}
	return emails, nil
}

// GetEmail returns a single email by UID from disk cache
func (sm *StateManager) GetEmail(email, mailbox string, uid imap.UID) (*cache.CachedEmail, error) {
	if sm.cache == nil {
		return nil, nil
	}
	return sm.cache.GetEmail(email, mailbox, uid)
}

// GetEmailWithBody loads an email from disk cache, fetching body from IMAP if missing.
func (sm *StateManager) GetEmailWithBody(email, mailbox string, uid imap.UID) (*cache.CachedEmail, error) {
	if sm.cache == nil {
		return nil, nil
	}

	cached, err := sm.cache.GetEmail(email, mailbox, uid)
	if err != nil {
		return nil, err
	}
	if cached == nil {
		return nil, nil
	}

	// Return if body already cached
	if cached.BodyHTML != "" || cached.Snippet != "" {
		return cached, nil
	}

	// Fetch body from IMAP and persist
	fetchErr := sm.withIMAPClient(email, func(client *mail.IMAPClient) error {
		bodyHTML, snippet, err := client.FetchEmailBody(mailbox, uid)
		if err != nil {
			return err
		}
		cached.BodyHTML = bodyHTML
		cached.Snippet = snippet
		return nil
	})
	if fetchErr != nil {
		// If email was deleted on server, remove from cache
		if errors.Is(fetchErr, mail.ErrEmailNotFound) {
			_ = sm.cache.DeleteEmail(email, mailbox, uid)
			return nil, fmt.Errorf("email was deleted on another device")
		}
		return cached, fetchErr
	}

	// Save body to disk cache
	if cached.BodyHTML != "" || cached.Snippet != "" {
		_ = sm.cache.UpdateEmailBody(email, mailbox, uid, cached.BodyHTML, cached.Snippet)
	}

	return cached, nil
}

// UpdateEmail updates an email in disk cache
func (sm *StateManager) UpdateEmail(email, mailbox string, cached cache.CachedEmail) error {
	if sm.cache == nil {
		return nil
	}
	return sm.cache.SaveEmail(email, mailbox, cached)
}

// UpdateEmailFlags updates only the unread flag in disk cache
func (sm *StateManager) UpdateEmailFlags(email, mailbox string, uid imap.UID, unread bool) error {
	if sm.cache == nil {
		return nil
	}
	return sm.cache.UpdateEmailFlags(email, mailbox, uid, unread)
}

// DeleteEmail removes an email from disk cache
func (sm *StateManager) DeleteEmail(email, mailbox string, uid imap.UID) error {
	if sm.cache == nil {
		return nil
	}
	return sm.cache.DeleteEmail(email, mailbox, uid)
}

// QueueOp deletes an email from cache and enqueues a pending operation.
func (sm *StateManager) QueueOp(account, mailbox, operation string, uid imap.UID) error {
	if sm.cache == nil {
		return fmt.Errorf("cache unavailable")
	}
	if _, err := sm.getAccountState(account); err != nil {
		return err
	}
	if err := sm.cache.DeleteEmail(account, mailbox, uid); err != nil {
		return err
	}
	return sm.cache.AddPendingOp(account, mailbox, operation, uid)
}

// QueueOps deletes multiple emails from cache and enqueues pending operations.
func (sm *StateManager) QueueOps(account, mailbox, operation string, uids []imap.UID) error {
	if len(uids) == 0 {
		return nil
	}
	if sm.cache == nil {
		return fmt.Errorf("cache unavailable")
	}
	if _, err := sm.getAccountState(account); err != nil {
		return err
	}
	for _, uid := range uids {
		if err := sm.cache.DeleteEmail(account, mailbox, uid); err != nil {
			return err
		}
		if err := sm.cache.AddPendingOp(account, mailbox, operation, uid); err != nil {
			return err
		}
	}
	return nil
}

// GetAccountCredentials returns credentials for an account
func (sm *StateManager) GetAccountCredentials(email string) (*auth.Credentials, error) {
	state, err := sm.getAccountState(email)
	if err != nil {
		return nil, err
	}
	return &state.Account.Credentials, nil
}

// GetLabels fetches labels from IMAP
func (sm *StateManager) GetLabels(email string) ([]string, error) {
	var labels []string
	err := sm.withIMAPClient(email, func(client *mail.IMAPClient) error {
		var err error
		labels, err = client.ListMailboxes()
		return err
	})
	if err != nil {
		return nil, err
	}
	return labels, nil
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
	var syncErr error
	defer func() {
		sm.EndSync(email, syncErr)
	}()

	syncErr = sm.withIMAPClient(email, func(client *mail.IMAPClient) error {
		var uidValidity uint32
		if info, err := client.SelectMailboxWithInfo(mailbox); err == nil {
			uidValidity = info.UIDValidity
		}

		// Step 1: Fetch last 100 emails by sequence number (metadata only, no body)
		emails, err := client.FetchMessagesMetadata(mailbox, MinSyncEmails)
		if err != nil {
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

		// Step 4: Fetch any missing recent emails (metadata only)
		if len(missingUIDs) > 0 {
			additional, err := client.FetchMessagesByUIDsMetadata(mailbox, missingUIDs)
			if err == nil {
				emails = append(emails, additional...)
			}
		}

		// Convert to cached format and persist to disk
		cached := make([]cache.CachedEmail, len(emails))
		for i, e := range emails {
			cached[i] = emailToCached(e)
		}

		// Persist to disk (insert metadata only if missing)
		if sm.cache != nil {
			for _, c := range cached {
				_, _ = sm.cache.InsertEmailMetadataIfMissing(email, mailbox, c)
			}

			// Step 5: Remove stale emails from disk cache
			// Build set of all server UIDs
			serverUIDs := make(map[imap.UID]bool)
			for _, e := range emails {
				serverUIDs[e.UID] = true
			}

			// Delete cached UIDs that no longer exist on server
			cachedUIDs, err := sm.cache.GetCachedUIDs(email, mailbox)
			if err == nil {
				for uid := range cachedUIDs {
					if !serverUIDs[uid] {
						_ = sm.cache.DeleteEmail(email, mailbox, uid)
					}
				}
			}

			// Step 6: Prefetch body for 10 most recent emails
			// (cached is already sorted by InternalDate desc)
			var prefetchUIDs []imap.UID
			for i := 0; i < len(cached) && len(prefetchUIDs) < 10; i++ {
				if cached[i].BodyHTML == "" {
					prefetchUIDs = append(prefetchUIDs, cached[i].UID)
				}
			}

			if len(prefetchUIDs) > 0 {
				fullEmails, err := client.FetchMessagesByUIDs(mailbox, prefetchUIDs)
				if err == nil {
					for _, fe := range fullEmails {
						_ = sm.cache.UpdateEmailBody(email, mailbox, fe.UID, fe.BodyHTML, fe.Snippet)
					}
				}
			}
		}

		if sm.cache != nil {
			if uidValidity == 0 {
				if meta, err := sm.cache.LoadMetadata(email, mailbox); err == nil && meta != nil {
					uidValidity = meta.UIDValidity
				}
			}
			_ = sm.cache.SaveMetadata(email, mailbox, &cache.Metadata{
				UIDValidity: uidValidity,
				LastSync:    time.Now(),
			})
		}

		return nil
	})

	return syncErr
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
		state, err := sm.getAccountState(account)
		if err != nil {
			// Mark all ops for this account as failed
			for _, op := range accountOps {
				sm.cache.UpdatePendingOpError(op.ID, err.Error())
				failed++
			}
			continue
		}

		state.imapMu.Lock()
		client, err := sm.ensureIMAPClientLocked(state)
		if err != nil {
			state.imapMu.Unlock()
			for _, op := range accountOps {
				sm.cache.UpdatePendingOpError(op.ID, err.Error())
				failed++
			}
			continue
		}

		for i, op := range accountOps {
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

				if isConnectionError(opErr) {
					client.Close()
					state.imapClient = nil
					client, err = sm.ensureIMAPClientLocked(state)
					if err != nil {
						for _, remaining := range accountOps[i+1:] {
							sm.cache.UpdatePendingOpError(remaining.ID, err.Error())
							failed++
						}
						break
					}
				}
				continue
			}

			sm.cache.RemovePendingOp(op.ID)
			sm.cache.LogOp(op, cache.StatusSuccess, "")
			// Delete from cache again in case sync pulled email back
			if op.Operation == cache.OpDelete || op.Operation == cache.OpMoveTrash {
				sm.cache.DeleteEmail(op.Account, op.Mailbox, op.UID)
			}
			processed++
		}

		state.imapMu.Unlock()
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
