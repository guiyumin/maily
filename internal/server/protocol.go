package server

import (
	"time"

	"github.com/emersion/go-imap/v2"
	"maily/internal/cache"
)

// Request types
const (
	ReqHello           = "hello"
	ReqGetEmails       = "get_emails"
	ReqGetEmail        = "get_email"
	ReqSync            = "sync"
	ReqQuickRefresh    = "quick_refresh"
	ReqMarkRead        = "mark_read"
	ReqMarkUnread      = "mark_unread"
	ReqMarkMultiRead   = "mark_multi_read"
	ReqDeleteEmail     = "delete_email"
	ReqDeleteMulti     = "delete_multi"
	ReqMoveToTrash     = "move_to_trash"
	ReqMoveMultiTrash  = "move_multi_trash"
	ReqQueueDelete      = "queue_delete"
	ReqQueueDeleteMulti = "queue_delete_multi"
	ReqQueueMoveTrash   = "queue_move_trash"
	ReqQueueMoveMultiTrash = "queue_move_multi_trash"
	ReqSearch          = "search"
	ReqGetLabels       = "get_labels"
	ReqGetSyncStatus   = "get_sync_status"
	ReqGetAccounts     = "get_accounts"
	ReqPing            = "ping"
	ReqShutdown        = "shutdown"
	// Synchronous operations (real-time, no queuing)
	ReqSaveDraft           = "save_draft"
	ReqDownloadAttachment  = "download_attachment"
)

// Request is the message sent from client to server
type Request struct {
	Type    string   `json:"type"`
	ID      string   `json:"id,omitempty"` // for request/response matching
	Version string   `json:"version,omitempty"` // client version for hello handshake
	Account string   `json:"account,omitempty"`
	Mailbox string   `json:"mailbox,omitempty"`
	UID     uint32   `json:"uid,omitempty"`
	UIDs    []uint32 `json:"uids,omitempty"`
	Query   string   `json:"query,omitempty"`  // for search
	Target  string   `json:"target,omitempty"` // for move operations
	Limit   int      `json:"limit,omitempty"`
	// For save_draft
	To      string `json:"to,omitempty"`
	Subject string `json:"subject,omitempty"`
	Body    string `json:"body,omitempty"`
	// For download_attachment
	PartID   string `json:"part_id,omitempty"`
	Filename string `json:"filename,omitempty"`
	Encoding string `json:"encoding,omitempty"`
}

// Response types
const (
	RespOK       = "ok"
	RespError    = "error"
	RespHello    = "hello"
	RespEmails   = "emails"
	RespEmail    = "email"
	RespLabels   = "labels"
	RespStatus   = "status"
	RespAccounts = "accounts"
	RespPong     = "pong"
)

// Response is the message sent from server to client
type Response struct {
	Type     string         `json:"type"`
	ID       string         `json:"id,omitempty"`
	Version  string         `json:"version,omitempty"` // server version for hello response
	Error    string         `json:"error,omitempty"`
	Emails   []cache.CachedEmail `json:"emails,omitempty"`
	Email    *cache.CachedEmail  `json:"email,omitempty"`
	Labels   []string       `json:"labels,omitempty"`
	Accounts []AccountInfo  `json:"accounts,omitempty"`
	Status   *SyncStatus    `json:"status,omitempty"`
	// For download_attachment
	FilePath string `json:"file_path,omitempty"`
}

// AccountInfo is a summary of account state
type AccountInfo struct {
	Email      string    `json:"email"`
	Provider   string    `json:"provider"`
	Syncing    bool      `json:"syncing"`
	LastSync   time.Time `json:"last_sync"`
	EmailCount int       `json:"email_count"`
}

// SyncStatus represents sync state for an account
type SyncStatus struct {
	Account   string    `json:"account"`
	Syncing   bool      `json:"syncing"`
	LastSync  time.Time `json:"last_sync"`
	LastError string    `json:"last_error,omitempty"`
}

// Event types for server → client push notifications
const (
	EventSyncStarted   = "sync_started"
	EventSyncCompleted = "sync_completed"
	EventSyncError     = "sync_error"
	EventNewEmails     = "new_emails"
	EventEmailUpdated  = "email_updated"
)

// Event is pushed from server to connected clients
type Event struct {
	Type    string   `json:"type"`
	Account string   `json:"account,omitempty"`
	Mailbox string   `json:"mailbox,omitempty"`
	UIDs    []imap.UID `json:"uids,omitempty"`
	Error   string   `json:"error,omitempty"`
}
