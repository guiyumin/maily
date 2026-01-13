package client

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/emersion/go-imap/v2"
	"maily/internal/cache"
	"maily/internal/server"
)

// Client connects to the maily server
type Client struct {
	conn    net.Conn
	reader  *bufio.Reader
	encoder *json.Encoder
	reqID   uint64
	pending map[string]chan server.Response
	events  chan server.Event
	mu      sync.Mutex
	closed  bool
}

// Connect creates a new client connection to the server
func Connect() (*Client, error) {
	sockPath := server.GetSocketPath()
	conn, err := net.Dial("unix", sockPath)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to server: %w (is the server running?)", err)
	}

	c := &Client{
		conn:    conn,
		reader:  bufio.NewReader(conn),
		encoder: json.NewEncoder(conn),
		pending: make(map[string]chan server.Response),
		events:  make(chan server.Event, 100),
	}

	// Start reader goroutine
	go c.readLoop()

	return c, nil
}

// Close closes the connection
func (c *Client) Close() error {
	c.mu.Lock()
	c.closed = true
	c.mu.Unlock()
	return c.conn.Close()
}

// Events returns the channel for server push events
func (c *Client) Events() <-chan server.Event {
	return c.events
}

// readLoop reads responses and events from the server
func (c *Client) readLoop() {
	for {
		line, err := c.reader.ReadBytes('\n')
		if err != nil {
			c.mu.Lock()
			closed := c.closed
			c.mu.Unlock()
			if !closed {
				// Unexpected disconnect
				close(c.events)
			}
			return
		}

		// Try to parse as response
		var resp server.Response
		if err := json.Unmarshal(line, &resp); err == nil && resp.Type != "" {
			if resp.ID != "" {
				c.mu.Lock()
				if ch, ok := c.pending[resp.ID]; ok {
					ch <- resp
					delete(c.pending, resp.ID)
				}
				c.mu.Unlock()
			}
			continue
		}

		// Try to parse as event
		var event server.Event
		if err := json.Unmarshal(line, &event); err == nil && event.Type != "" {
			select {
			case c.events <- event:
			default:
				// Event buffer full
			}
		}
	}
}

// request sends a request and waits for response
func (c *Client) request(req server.Request, timeout time.Duration) (server.Response, error) {
	// Generate unique request ID
	id := fmt.Sprintf("%d", atomic.AddUint64(&c.reqID, 1))
	req.ID = id

	// Create response channel
	respChan := make(chan server.Response, 1)
	c.mu.Lock()
	c.pending[id] = respChan
	c.mu.Unlock()

	// Send request
	if err := c.encoder.Encode(req); err != nil {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return server.Response{}, fmt.Errorf("failed to send request: %w", err)
	}

	// Wait for response
	select {
	case resp := <-respChan:
		if resp.Type == server.RespError {
			return resp, fmt.Errorf("%s", resp.Error)
		}
		return resp, nil
	case <-time.After(timeout):
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return server.Response{}, fmt.Errorf("request timed out")
	}
}

// Ping checks if the server is responsive
func (c *Client) Ping() error {
	_, err := c.request(server.Request{Type: server.ReqPing}, 5*time.Second)
	return err
}

// GetAccounts returns all configured accounts
func (c *Client) GetAccounts() ([]server.AccountInfo, error) {
	resp, err := c.request(server.Request{Type: server.ReqGetAccounts}, 10*time.Second)
	if err != nil {
		return nil, err
	}
	return resp.Accounts, nil
}

// GetEmails returns emails for an account/mailbox
func (c *Client) GetEmails(account, mailbox string, limit int) ([]cache.CachedEmail, error) {
	resp, err := c.request(server.Request{
		Type:    server.ReqGetEmails,
		Account: account,
		Mailbox: mailbox,
		Limit:   limit,
	}, 30*time.Second)
	if err != nil {
		return nil, err
	}
	return resp.Emails, nil
}

// GetEmail returns a single email by UID
func (c *Client) GetEmail(account, mailbox string, uid imap.UID) (*cache.CachedEmail, error) {
	resp, err := c.request(server.Request{
		Type:    server.ReqGetEmail,
		Account: account,
		Mailbox: mailbox,
		UID:     uint32(uid),
	}, 10*time.Second)
	if err != nil {
		return nil, err
	}
	return resp.Email, nil
}

// GetLabels returns mailbox labels for an account
func (c *Client) GetLabels(account string) ([]string, error) {
	resp, err := c.request(server.Request{
		Type:    server.ReqGetLabels,
		Account: account,
	}, 30*time.Second)
	if err != nil {
		return nil, err
	}
	return resp.Labels, nil
}

// GetSyncStatus returns sync status for an account
func (c *Client) GetSyncStatus(account string) (*server.SyncStatus, error) {
	resp, err := c.request(server.Request{
		Type:    server.ReqGetSyncStatus,
		Account: account,
	}, 10*time.Second)
	if err != nil {
		return nil, err
	}
	return resp.Status, nil
}

// Sync triggers a sync for an account/mailbox (non-blocking)
func (c *Client) Sync(account, mailbox string) error {
	_, err := c.request(server.Request{
		Type:    server.ReqSync,
		Account: account,
		Mailbox: mailbox,
	}, 10*time.Second)
	return err
}

// MarkRead marks an email as read
func (c *Client) MarkRead(account, mailbox string, uid imap.UID) error {
	_, err := c.request(server.Request{
		Type:    server.ReqMarkRead,
		Account: account,
		Mailbox: mailbox,
		UID:     uint32(uid),
	}, 30*time.Second)
	return err
}

// MarkUnread marks an email as unread
func (c *Client) MarkUnread(account, mailbox string, uid imap.UID) error {
	_, err := c.request(server.Request{
		Type:    server.ReqMarkUnread,
		Account: account,
		Mailbox: mailbox,
		UID:     uint32(uid),
	}, 30*time.Second)
	return err
}

// DeleteEmail deletes an email
func (c *Client) DeleteEmail(account, mailbox string, uid imap.UID) error {
	_, err := c.request(server.Request{
		Type:    server.ReqDeleteEmail,
		Account: account,
		Mailbox: mailbox,
		UID:     uint32(uid),
	}, 30*time.Second)
	return err
}

// DeleteMulti deletes multiple emails
func (c *Client) DeleteMulti(account, mailbox string, uids []imap.UID) error {
	uint32UIDs := make([]uint32, len(uids))
	for i, uid := range uids {
		uint32UIDs[i] = uint32(uid)
	}
	_, err := c.request(server.Request{
		Type:    server.ReqDeleteMulti,
		Account: account,
		Mailbox: mailbox,
		UIDs:    uint32UIDs,
	}, 30*time.Second)
	return err
}

// MoveToTrash moves an email to trash
func (c *Client) MoveToTrash(account, mailbox string, uid imap.UID) error {
	_, err := c.request(server.Request{
		Type:    server.ReqMoveToTrash,
		Account: account,
		Mailbox: mailbox,
		UID:     uint32(uid),
	}, 30*time.Second)
	return err
}

// MoveMultiToTrash moves multiple emails to trash
func (c *Client) MoveMultiToTrash(account, mailbox string, uids []imap.UID) error {
	uint32UIDs := make([]uint32, len(uids))
	for i, uid := range uids {
		uint32UIDs[i] = uint32(uid)
	}
	_, err := c.request(server.Request{
		Type:    server.ReqMoveMultiTrash,
		Account: account,
		Mailbox: mailbox,
		UIDs:    uint32UIDs,
	}, 30*time.Second)
	return err
}

// MarkMultiRead marks multiple emails as read
func (c *Client) MarkMultiRead(account, mailbox string, uids []imap.UID) error {
	uint32UIDs := make([]uint32, len(uids))
	for i, uid := range uids {
		uint32UIDs[i] = uint32(uid)
	}
	_, err := c.request(server.Request{
		Type:    server.ReqMarkMultiRead,
		Account: account,
		Mailbox: mailbox,
		UIDs:    uint32UIDs,
	}, 30*time.Second)
	return err
}

// Search searches for emails
func (c *Client) Search(account, mailbox, query string) ([]cache.CachedEmail, error) {
	resp, err := c.request(server.Request{
		Type:    server.ReqSearch,
		Account: account,
		Mailbox: mailbox,
		Query:   query,
	}, 60*time.Second)
	if err != nil {
		return nil, err
	}
	return resp.Emails, nil
}

// Shutdown tells the server to shut down
func (c *Client) Shutdown() error {
	_, err := c.request(server.Request{Type: server.ReqShutdown}, 5*time.Second)
	return err
}
