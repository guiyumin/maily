package server

import (
	"sort"
	"sync"

	"github.com/emersion/go-imap/v2"
	"maily/internal/cache"
)

// MemoryCache provides fast in-memory email storage
type MemoryCache struct {
	// emails[account][mailbox] = []CachedEmail
	emails map[string]map[string][]cache.CachedEmail
	mu     sync.RWMutex
}

// NewMemoryCache creates a new memory cache
func NewMemoryCache() *MemoryCache {
	return &MemoryCache{
		emails: make(map[string]map[string][]cache.CachedEmail),
	}
}

// Get returns all emails for account/mailbox
func (mc *MemoryCache) Get(account, mailbox string) []cache.CachedEmail {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	if acc, ok := mc.emails[account]; ok {
		if emails, ok := acc[mailbox]; ok {
			// Return a copy to avoid race conditions
			result := make([]cache.CachedEmail, len(emails))
			copy(result, emails)
			return result
		}
	}
	return nil
}

// GetByUID returns a single email by UID
func (mc *MemoryCache) GetByUID(account, mailbox string, uid imap.UID) *cache.CachedEmail {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	if acc, ok := mc.emails[account]; ok {
		if emails, ok := acc[mailbox]; ok {
			for i := range emails {
				if emails[i].UID == uid {
					// Return a copy
					result := emails[i]
					return &result
				}
			}
		}
	}
	return nil
}

// Set stores emails for account/mailbox (replaces existing)
func (mc *MemoryCache) Set(account, mailbox string, emails []cache.CachedEmail) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	if mc.emails[account] == nil {
		mc.emails[account] = make(map[string][]cache.CachedEmail)
	}

	// Sort by InternalDate descending (newest first)
	sorted := make([]cache.CachedEmail, len(emails))
	copy(sorted, emails)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].InternalDate.After(sorted[j].InternalDate)
	})

	mc.emails[account][mailbox] = sorted
}

// Update updates a single email in place
func (mc *MemoryCache) Update(account, mailbox string, email cache.CachedEmail) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	if acc, ok := mc.emails[account]; ok {
		if emails, ok := acc[mailbox]; ok {
			for i := range emails {
				if emails[i].UID == email.UID {
					emails[i] = email
					return
				}
			}
			// Not found, append
			mc.emails[account][mailbox] = append(emails, email)
		}
	}
}

// Delete removes an email by UID
func (mc *MemoryCache) Delete(account, mailbox string, uid imap.UID) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	if acc, ok := mc.emails[account]; ok {
		if emails, ok := acc[mailbox]; ok {
			for i := range emails {
				if emails[i].UID == uid {
					mc.emails[account][mailbox] = append(emails[:i], emails[i+1:]...)
					return
				}
			}
		}
	}
}

// Count returns number of emails for account/mailbox
func (mc *MemoryCache) Count(account, mailbox string) int {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	if acc, ok := mc.emails[account]; ok {
		if emails, ok := acc[mailbox]; ok {
			return len(emails)
		}
	}
	return 0
}

// Clear removes all emails for an account
func (mc *MemoryCache) Clear(account string) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	delete(mc.emails, account)
}

// ClearMailbox removes all emails for a specific mailbox
func (mc *MemoryCache) ClearMailbox(account, mailbox string) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	if acc, ok := mc.emails[account]; ok {
		delete(acc, mailbox)
	}
}

// Stats returns cache statistics
func (mc *MemoryCache) Stats() map[string]int {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	stats := make(map[string]int)
	total := 0
	for account, mailboxes := range mc.emails {
		accountTotal := 0
		for _, emails := range mailboxes {
			accountTotal += len(emails)
		}
		stats[account] = accountTotal
		total += accountTotal
	}
	stats["total"] = total
	return stats
}
