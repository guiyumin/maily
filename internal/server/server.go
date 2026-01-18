package server

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"maily/internal/auth"
	"maily/internal/cache"
	"maily/internal/mail"
	"maily/internal/version"

	"github.com/emersion/go-imap/v2"
)

const (
	syncInterval = 10 * time.Minute
)

// Server is the long-running maily server process
type Server struct {
	sockPath string
	listener net.Listener
	state    *StateManager
	clients  map[*Client]bool
	clientMu sync.RWMutex
	done     chan struct{}
	wg       sync.WaitGroup
}

// Client represents a connected TUI client
type Client struct {
	conn   net.Conn
	server *Server
	events chan Event
}

// GetSocketPath returns the default socket path
func GetSocketPath() string {
	homeDir, _ := os.UserHomeDir()
	return filepath.Join(homeDir, ".config", "maily", "maily.sock")
}

// GetPidPath returns the server PID file path
func GetPidPath() string {
	homeDir, _ := os.UserHomeDir()
	return filepath.Join(homeDir, ".config", "maily", "server.pid")
}

// IsServerRunning checks if a server is already running
func IsServerRunning() bool {
	sockPath := GetSocketPath()
	conn, err := net.Dial("unix", sockPath)
	if err != nil {
		// Clean up stale socket
		os.Remove(sockPath)
		return false
	}
	conn.Close()
	return true
}

// New creates a new server instance
func New() (*Server, error) {
	sockPath := GetSocketPath()

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(sockPath), 0700); err != nil {
		return nil, fmt.Errorf("failed to create socket directory: %w", err)
	}

	// Remove stale socket
	os.Remove(sockPath)

	// Load accounts
	store, err := auth.LoadAccountStore()
	if err != nil {
		return nil, fmt.Errorf("failed to load accounts: %w", err)
	}

	if len(store.Accounts) == 0 {
		return nil, fmt.Errorf("no accounts configured - run 'maily login' first")
	}

	// Initialize disk cache
	diskCache, err := cache.New()
	if err != nil {
		return nil, fmt.Errorf("failed to create cache: %w", err)
	}

	// Create listener
	listener, err := net.Listen("unix", sockPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create socket: %w", err)
	}

	// Set socket permissions
	os.Chmod(sockPath, 0600)

	return &Server{
		sockPath: sockPath,
		listener: listener,
		state:    NewStateManager(store, diskCache),
		clients:  make(map[*Client]bool),
		done:     make(chan struct{}),
	}, nil
}

// Run starts the server and blocks until shutdown
func (s *Server) Run() error {
	// Write PID file
	pidPath := GetPidPath()
	pidContent := fmt.Sprintf("%d:%s", os.Getpid(), version.Version)
	if err := os.WriteFile(pidPath, []byte(pidContent), 0600); err != nil {
		return fmt.Errorf("failed to write PID file: %w", err)
	}
	defer os.Remove(pidPath)

	fmt.Printf("Server started (PID %d, socket %s)\n", os.Getpid(), s.sockPath)

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start background poller
	s.wg.Add(1)
	go s.backgroundPoller()

	// Start accepting connections
	s.wg.Add(1)
	go s.acceptLoop()

	// Initial sync (skip if cache is fresh)
	s.syncAllAccountsIfStale(syncInterval)

	// Wait for shutdown signal
	<-sigChan
	fmt.Println("\nShutting down...")

	// Close listener (stops accept loop)
	s.listener.Close()

	// Signal all goroutines to stop
	close(s.done)

	// Close all client connections
	s.clientMu.Lock()
	for client := range s.clients {
		client.conn.Close()
	}
	s.clientMu.Unlock()

	// Wait for goroutines to finish
	s.wg.Wait()

	// Close any pooled IMAP clients
	s.state.CloseIMAPClients()

	// Clean up socket
	os.Remove(s.sockPath)

	fmt.Println("Server stopped")
	return nil
}

// acceptLoop accepts new client connections
func (s *Server) acceptLoop() {
	defer s.wg.Done()

	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.done:
				return // Normal shutdown
			default:
				fmt.Printf("Accept error: %v\n", err)
				continue
			}
		}

		client := &Client{
			conn:   conn,
			server: s,
			events: make(chan Event, 100),
		}

		s.clientMu.Lock()
		s.clients[client] = true
		s.clientMu.Unlock()

		s.wg.Add(1)
		go s.handleClient(client)
	}
}

// handleClient handles a single client connection
func (s *Server) handleClient(client *Client) {
	defer s.wg.Done()
	defer func() {
		s.clientMu.Lock()
		delete(s.clients, client)
		s.clientMu.Unlock()
		client.conn.Close()
		close(client.events)
	}()

	reader := bufio.NewReader(client.conn)
	encoder := json.NewEncoder(client.conn)

	// Start event sender goroutine
	go func() {
		for event := range client.events {
			data, _ := json.Marshal(event)
			client.conn.Write(append(data, '\n'))
		}
	}()

	for {
		// Read request
		line, err := reader.ReadBytes('\n')
		if err != nil {
			return // Client disconnected
		}

		var req Request
		if err := json.Unmarshal(line, &req); err != nil {
			encoder.Encode(Response{Type: RespError, ID: req.ID, Error: "invalid request"})
			continue
		}

		// Handle request
		resp := s.handleRequest(client, &req)
		resp.ID = req.ID
		encoder.Encode(resp)
	}
}

// handleRequest processes a single request
func (s *Server) handleRequest(_ *Client, req *Request) Response {
	switch req.Type {
	case ReqHello:
		serverVersion := version.Version
		clientVersion := req.Version
		// Check major.minor match (ignore patch)
		if !versionsCompatible(serverVersion, clientVersion) {
			return Response{
				Type:    RespError,
				Version: serverVersion,
				Error:   fmt.Sprintf("version mismatch: server=%s, client=%s - please restart maily", serverVersion, clientVersion),
			}
		}
		return Response{Type: RespHello, Version: serverVersion}

	case ReqPing:
		return Response{Type: RespPong}

	case ReqGetAccounts:
		accounts := s.state.GetAccounts()
		return Response{Type: RespAccounts, Accounts: accounts}

	case ReqGetEmails:
		emails, err := s.state.GetEmails(req.Account, req.Mailbox, req.Limit)
		if err != nil {
			return Response{Type: RespError, Error: err.Error()}
		}
		return Response{Type: RespEmails, Emails: emails}

	case ReqGetEmail:
		email, err := s.state.GetEmailWithBody(req.Account, req.Mailbox, imap.UID(req.UID))
		if err != nil {
			return Response{Type: RespError, Error: err.Error()}
		}
		return Response{Type: RespEmail, Email: email}

	case ReqGetLabels:
		labels, err := s.state.GetLabels(req.Account)
		if err != nil {
			return Response{Type: RespError, Error: err.Error()}
		}
		return Response{Type: RespLabels, Labels: labels}

	case ReqGetSyncStatus:
		status, err := s.state.GetSyncStatus(req.Account)
		if err != nil {
			return Response{Type: RespError, Error: err.Error()}
		}
		return Response{Type: RespStatus, Status: status}

	case ReqSync:
		go func() {
			s.broadcastEvent(Event{Type: EventSyncStarted, Account: req.Account})
			err := s.state.Sync(req.Account, req.Mailbox)
			if err != nil {
				s.broadcastEvent(Event{Type: EventSyncError, Account: req.Account, Error: err.Error()})
			} else {
				s.broadcastEvent(Event{Type: EventSyncCompleted, Account: req.Account})
			}
		}()
		return Response{Type: RespOK}

	case ReqMarkRead:
		return s.markEmailRead(req.Account, req.Mailbox, imap.UID(req.UID), true)

	case ReqMarkUnread:
		return s.markEmailRead(req.Account, req.Mailbox, imap.UID(req.UID), false)

	case ReqDeleteEmail:
		return s.deleteEmail(req.Account, req.Mailbox, imap.UID(req.UID))

	case ReqDeleteMulti:
		return s.deleteMultiEmails(req.Account, req.Mailbox, req.UIDs)

	case ReqMoveToTrash:
		return s.moveToTrash(req.Account, req.Mailbox, imap.UID(req.UID))

	case ReqMoveMultiTrash:
		return s.moveMultiToTrash(req.Account, req.Mailbox, req.UIDs)

	case ReqQueueDelete:
		return s.queueDeleteEmail(req.Account, req.Mailbox, imap.UID(req.UID))

	case ReqQueueDeleteMulti:
		return s.queueDeleteMultiEmails(req.Account, req.Mailbox, req.UIDs)

	case ReqQueueMoveTrash:
		return s.queueMoveToTrash(req.Account, req.Mailbox, imap.UID(req.UID))

	case ReqQueueMoveMultiTrash:
		return s.queueMoveMultiToTrash(req.Account, req.Mailbox, req.UIDs)

	case ReqMarkMultiRead:
		return s.markMultiRead(req.Account, req.Mailbox, req.UIDs)

	case ReqSearch:
		return s.searchEmails(req.Account, req.Mailbox, req.Query)

	case ReqShutdown:
		go func() {
			time.Sleep(100 * time.Millisecond)
			syscall.Kill(os.Getpid(), syscall.SIGTERM)
		}()
		return Response{Type: RespOK}

	default:
		return Response{Type: RespError, Error: "unknown request type"}
	}
}

// markEmailRead updates read status on IMAP and cache
func (s *Server) markEmailRead(account, mailbox string, uid imap.UID, read bool) Response {
	err := s.state.withIMAPClient(account, func(client *mail.IMAPClient) error {
		if err := client.SelectMailbox(mailbox); err != nil {
			return err
		}
		if read {
			return client.MarkAsRead(uid)
		}
		return client.MarkAsUnread(uid)
	})
	if err != nil {
		return Response{Type: RespError, Error: err.Error()}
	}

	_ = s.state.UpdateEmailFlags(account, mailbox, uid, !read)
	return Response{Type: RespOK}
}

// deleteEmail deletes an email from IMAP and cache
func (s *Server) deleteEmail(account, mailbox string, uid imap.UID) Response {
	err := s.state.withIMAPClient(account, func(client *mail.IMAPClient) error {
		if err := client.SelectMailbox(mailbox); err != nil {
			return err
		}
		return client.DeleteMessage(uid)
	})
	if err != nil {
		return Response{Type: RespError, Error: err.Error()}
	}

	// Update cache
	s.state.DeleteEmail(account, mailbox, uid)

	return Response{Type: RespOK}
}

// queueDeleteEmail deletes an email from cache and enqueues a delete op.
func (s *Server) queueDeleteEmail(account, mailbox string, uid imap.UID) Response {
	if err := s.state.QueueOp(account, mailbox, cache.OpDelete, uid); err != nil {
		return Response{Type: RespError, Error: err.Error()}
	}
	return Response{Type: RespOK}
}

// queueDeleteMultiEmails deletes multiple emails from cache and enqueues delete ops.
func (s *Server) queueDeleteMultiEmails(account, mailbox string, uids []uint32) Response {
	if len(uids) == 0 {
		return Response{Type: RespOK}
	}
	imapUIDs := make([]imap.UID, len(uids))
	for i, uid := range uids {
		imapUIDs[i] = imap.UID(uid)
	}
	if err := s.state.QueueOps(account, mailbox, cache.OpDelete, imapUIDs); err != nil {
		return Response{Type: RespError, Error: err.Error()}
	}
	return Response{Type: RespOK}
}

// broadcastEvent sends an event to all connected clients
func (s *Server) broadcastEvent(event Event) {
	s.clientMu.RLock()
	defer s.clientMu.RUnlock()

	for client := range s.clients {
		select {
		case client.events <- event:
		default:
			// Client event buffer full, skip
		}
	}
}

// backgroundPoller syncs all accounts periodically and processes pending ops
func (s *Server) backgroundPoller() {
	defer s.wg.Done()

	syncTicker := time.NewTicker(syncInterval)
	defer syncTicker.Stop()

	// Process pending ops more frequently (every 10 seconds)
	opsTicker := time.NewTicker(10 * time.Second)
	defer opsTicker.Stop()

	for {
		select {
		case <-syncTicker.C:
			s.syncAllAccounts()
		case <-opsTicker.C:
			s.processPendingOps()
		case <-s.done:
			return
		}
	}
}

// processPendingOps processes the pending operations queue
func (s *Server) processPendingOps() {
	processed, failed := s.state.ProcessPendingOps()
	if processed > 0 || failed > 0 {
		fmt.Printf("Pending ops: %d processed, %d failed\n", processed, failed)
	}
}

// syncAllAccounts syncs INBOX for all accounts
func (s *Server) syncAllAccounts() {
	accounts := s.state.GetAccounts()
	for _, acc := range accounts {
		s.broadcastEvent(Event{Type: EventSyncStarted, Account: acc.Email})
		err := s.state.Sync(acc.Email, "INBOX")
		if err != nil {
			fmt.Printf("Sync error for %s: %v\n", acc.Email, err)
			s.broadcastEvent(Event{Type: EventSyncError, Account: acc.Email, Error: err.Error()})
		} else {
			fmt.Printf("Synced %s\n", acc.Email)
			s.broadcastEvent(Event{Type: EventSyncCompleted, Account: acc.Email})
		}
	}
}

// syncAllAccountsIfStale syncs INBOX for accounts without a recent cache.
func (s *Server) syncAllAccountsIfStale(maxAge time.Duration) {
	accounts := s.state.GetAccounts()
	for _, acc := range accounts {
		if s.state.IsCacheFresh(acc.Email, "INBOX", maxAge) {
			fmt.Printf("Skipping initial sync for %s (cache fresh)\n", acc.Email)
			continue
		}
		s.broadcastEvent(Event{Type: EventSyncStarted, Account: acc.Email})
		err := s.state.Sync(acc.Email, "INBOX")
		if err != nil {
			fmt.Printf("Sync error for %s: %v\n", acc.Email, err)
			s.broadcastEvent(Event{Type: EventSyncError, Account: acc.Email, Error: err.Error()})
		} else {
			fmt.Printf("Synced %s\n", acc.Email)
			s.broadcastEvent(Event{Type: EventSyncCompleted, Account: acc.Email})
		}
	}
}

// deleteMultiEmails deletes multiple emails from IMAP and cache
func (s *Server) deleteMultiEmails(account, mailbox string, uids []uint32) Response {
	if len(uids) == 0 {
		return Response{Type: RespOK}
	}

	// Convert to imap.UID slice
	imapUIDs := make([]imap.UID, len(uids))
	for i, uid := range uids {
		imapUIDs[i] = imap.UID(uid)
	}

	err := s.state.withIMAPClient(account, func(client *mail.IMAPClient) error {
		if err := client.SelectMailbox(mailbox); err != nil {
			return err
		}
		return client.DeleteMessages(imapUIDs)
	})
	if err != nil {
		return Response{Type: RespError, Error: err.Error()}
	}

	// Update cache
	for _, uid := range imapUIDs {
		s.state.DeleteEmail(account, mailbox, uid)
	}

	return Response{Type: RespOK}
}

// moveToTrash moves a single email to trash
func (s *Server) moveToTrash(account, mailbox string, uid imap.UID) Response {
	err := s.state.withIMAPClient(account, func(client *mail.IMAPClient) error {
		return client.MoveToTrashFromMailbox([]imap.UID{uid}, mailbox)
	})
	if err != nil {
		return Response{Type: RespError, Error: err.Error()}
	}

	// Update cache
	s.state.DeleteEmail(account, mailbox, uid)

	return Response{Type: RespOK}
}

// queueMoveToTrash deletes an email from cache and enqueues a move-to-trash op.
func (s *Server) queueMoveToTrash(account, mailbox string, uid imap.UID) Response {
	if err := s.state.QueueOp(account, mailbox, cache.OpMoveTrash, uid); err != nil {
		return Response{Type: RespError, Error: err.Error()}
	}
	return Response{Type: RespOK}
}

// moveMultiToTrash moves multiple emails to trash
func (s *Server) moveMultiToTrash(account, mailbox string, uids []uint32) Response {
	if len(uids) == 0 {
		return Response{Type: RespOK}
	}

	// Convert to imap.UID slice
	imapUIDs := make([]imap.UID, len(uids))
	for i, uid := range uids {
		imapUIDs[i] = imap.UID(uid)
	}

	err := s.state.withIMAPClient(account, func(client *mail.IMAPClient) error {
		return client.MoveToTrashFromMailbox(imapUIDs, mailbox)
	})
	if err != nil {
		return Response{Type: RespError, Error: err.Error()}
	}

	// Update cache
	for _, uid := range imapUIDs {
		s.state.DeleteEmail(account, mailbox, uid)
	}

	return Response{Type: RespOK}
}

// queueMoveMultiToTrash deletes multiple emails from cache and enqueues move-to-trash ops.
func (s *Server) queueMoveMultiToTrash(account, mailbox string, uids []uint32) Response {
	if len(uids) == 0 {
		return Response{Type: RespOK}
	}
	imapUIDs := make([]imap.UID, len(uids))
	for i, uid := range uids {
		imapUIDs[i] = imap.UID(uid)
	}
	if err := s.state.QueueOps(account, mailbox, cache.OpMoveTrash, imapUIDs); err != nil {
		return Response{Type: RespError, Error: err.Error()}
	}
	return Response{Type: RespOK}
}

// markMultiRead marks multiple emails as read
func (s *Server) markMultiRead(account, mailbox string, uids []uint32) Response {
	if len(uids) == 0 {
		return Response{Type: RespOK}
	}

	// Convert to imap.UID slice
	imapUIDs := make([]imap.UID, len(uids))
	for i, uid := range uids {
		imapUIDs[i] = imap.UID(uid)
	}

	err := s.state.withIMAPClient(account, func(client *mail.IMAPClient) error {
		if err := client.SelectMailbox(mailbox); err != nil {
			return err
		}
		return client.MarkMessagesAsRead(imapUIDs)
	})
	if err != nil {
		return Response{Type: RespError, Error: err.Error()}
	}

	// Update cache
	for _, uid := range imapUIDs {
		_ = s.state.UpdateEmailFlags(account, mailbox, uid, false)
	}

	return Response{Type: RespOK}
}

// searchEmails searches emails via IMAP
func (s *Server) searchEmails(account, mailbox, query string) Response {
	var emails []mail.Email
	err := s.state.withIMAPClient(account, func(client *mail.IMAPClient) error {
		var err error
		emails, err = client.SearchMessages(mailbox, query)
		return err
	})
	if err != nil {
		return Response{Type: RespError, Error: err.Error()}
	}

	// Convert to cached format
	cached := make([]cache.CachedEmail, len(emails))
	for i, e := range emails {
		cached[i] = emailToCached(e)
	}

	return Response{Type: RespEmails, Emails: cached}
}

// versionsCompatible checks if client and server versions match
func versionsCompatible(serverVer, clientVer string) bool {
	return serverVer == clientVer
}
