package mail

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/emersion/go-imap/v2"
	"maily/internal/auth"
)

// Search performs a search using the appropriate method for the provider
// For Gmail, it uses X-GM-RAW extension. For others, it uses standard IMAP SEARCH.
func Search(creds *auth.Credentials, mailbox string, query string) ([]imap.UID, error) {
	if strings.Contains(creds.IMAPHost, "gmail") {
		return GmailSearch(creds, mailbox, query)
	}
	return StandardSearch(creds, mailbox, query)
}

// StandardSearch performs a search using standard IMAP SEARCH command
// Supports basic text search in message body and headers
func StandardSearch(creds *auth.Credentials, mailbox string, query string) ([]imap.UID, error) {
	addr := fmt.Sprintf("%s:%d", creds.IMAPHost, creds.IMAPPort)

	conn, err := tls.Dial("tcp", addr, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}
	defer conn.Close()

	reader := bufio.NewReader(conn)

	// Read greeting
	if _, err := reader.ReadString('\n'); err != nil {
		return nil, fmt.Errorf("failed to read greeting: %w", err)
	}

	// Login
	loginCmd := fmt.Sprintf("a1 LOGIN %s %s\r\n", quoteString(creds.Email), quoteString(creds.Password))
	if _, err := conn.Write([]byte(loginCmd)); err != nil {
		return nil, fmt.Errorf("failed to send login: %w", err)
	}
	if err := readUntilOK(reader, "a1"); err != nil {
		return nil, fmt.Errorf("login failed: %w", err)
	}

	// Select mailbox
	selectCmd := fmt.Sprintf("a2 SELECT %s\r\n", quoteString(mailbox))
	if _, err := conn.Write([]byte(selectCmd)); err != nil {
		return nil, fmt.Errorf("failed to send select: %w", err)
	}
	if err := readUntilOK(reader, "a2"); err != nil {
		return nil, fmt.Errorf("select failed: %w", err)
	}

	// UID SEARCH with TEXT (searches body and headers)
	searchCmd := fmt.Sprintf("a3 UID SEARCH TEXT %s\r\n", quoteString(query))
	if _, err := conn.Write([]byte(searchCmd)); err != nil {
		return nil, fmt.Errorf("failed to send search: %w", err)
	}

	uids, err := readSearchResponse(reader, "a3")
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	// Logout
	conn.Write([]byte("a4 LOGOUT\r\n"))

	return uids, nil
}

// GmailSearch performs a search using Gmail's X-GM-RAW extension
// This allows using Gmail's native search syntax (from:, has:attachment, category:, etc.)
func GmailSearch(creds *auth.Credentials, mailbox string, query string) ([]imap.UID, error) {
	addr := fmt.Sprintf("%s:%d", creds.IMAPHost, creds.IMAPPort)

	conn, err := tls.Dial("tcp", addr, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}
	defer conn.Close()

	reader := bufio.NewReader(conn)

	// Read greeting
	if _, err := reader.ReadString('\n'); err != nil {
		return nil, fmt.Errorf("failed to read greeting: %w", err)
	}

	// Login
	loginCmd := fmt.Sprintf("a1 LOGIN %s %s\r\n", quoteString(creds.Email), quoteString(creds.Password))
	if _, err := conn.Write([]byte(loginCmd)); err != nil {
		return nil, fmt.Errorf("failed to send login: %w", err)
	}
	if err := readUntilOK(reader, "a1"); err != nil {
		return nil, fmt.Errorf("login failed: %w", err)
	}

	// Select mailbox
	selectCmd := fmt.Sprintf("a2 SELECT %s\r\n", quoteString(mailbox))
	if _, err := conn.Write([]byte(selectCmd)); err != nil {
		return nil, fmt.Errorf("failed to send select: %w", err)
	}
	if err := readUntilOK(reader, "a2"); err != nil {
		return nil, fmt.Errorf("select failed: %w", err)
	}

	// UID SEARCH with X-GM-RAW
	searchCmd := fmt.Sprintf("a3 UID SEARCH X-GM-RAW %s\r\n", quoteString(query))
	if _, err := conn.Write([]byte(searchCmd)); err != nil {
		return nil, fmt.Errorf("failed to send search: %w", err)
	}

	uids, err := readSearchResponse(reader, "a3")
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	// Logout
	conn.Write([]byte("a4 LOGOUT\r\n"))

	return uids, nil
}

func quoteString(s string) string {
	// Escape backslashes and quotes
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	return "\"" + s + "\""
}

func readUntilOK(reader *bufio.Reader, tag string) error {
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return err
		}
		line = strings.TrimSpace(line)

		if strings.HasPrefix(line, tag+" OK") {
			return nil
		}
		if strings.HasPrefix(line, tag+" NO") || strings.HasPrefix(line, tag+" BAD") {
			return fmt.Errorf("command failed: %s", line)
		}
	}
}

func readSearchResponse(reader *bufio.Reader, tag string) ([]imap.UID, error) {
	var uids []imap.UID
	uidRegex := regexp.MustCompile(`\* SEARCH(?: (\d+(?:\s+\d+)*))?`)

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimSpace(line)

		// Parse SEARCH response
		if matches := uidRegex.FindStringSubmatch(line); matches != nil {
			if len(matches) > 1 && matches[1] != "" {
				for _, numStr := range strings.Fields(matches[1]) {
					num, err := strconv.ParseUint(numStr, 10, 32)
					if err == nil {
						uids = append(uids, imap.UID(num))
					}
				}
			}
		}

		if strings.HasPrefix(line, tag+" OK") {
			return uids, nil
		}
		if strings.HasPrefix(line, tag+" NO") || strings.HasPrefix(line, tag+" BAD") {
			return nil, fmt.Errorf("search failed: %s", line)
		}
	}
}
