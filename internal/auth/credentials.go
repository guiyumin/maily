package auth

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"golang.org/x/term"
	"gopkg.in/yaml.v3"
)

const accountsFileName = "accounts.yml"

// Provider identifiers
const (
	ProviderGmail = "gmail"
	ProviderYahoo = "yahoo"
	ProviderQQ    = "qq"
)

// Gmail IMAP/SMTP hosts
const (
	GmailIMAPHost = "imap.gmail.com"
	GmailSMTPHost = "smtp.gmail.com"
)

// Yahoo IMAP/SMTP hosts
const (
	YahooIMAPHost = "imap.mail.yahoo.com"
	YahooSMTPHost = "smtp.mail.yahoo.com"
)

// QQ Mail IMAP/SMTP hosts
const (
	QQIMAPHost = "imap.qq.com"
	QQSMTPHost = "smtp.qq.com"
)

// QQ Mail uses port 465 for SMTP with SSL
const (
	QQSMTPPort = 465
)

// Standard ports
const (
	IMAPPort = 993
	SMTPPort = 587
)

type Credentials struct {
	Email    string `yaml:"email"`
	Password string `yaml:"password"`
	IMAPHost string `yaml:"imap_host"`
	IMAPPort int    `yaml:"imap_port"`
	SMTPHost string `yaml:"smtp_host"`
	SMTPPort int    `yaml:"smtp_port"`
	Provider string `yaml:"provider"`
}

type Account struct {
	Name        string      `yaml:"name"`
	Provider    string      `yaml:"provider"`
	Credentials Credentials `yaml:"credentials"`
	Avatar      string      `yaml:"avatar,omitempty"`
}

type AccountStore struct {
	Accounts []Account `yaml:"accounts"`
}

func GmailCredentials(email, password string) Credentials {
	return Credentials{
		Email:    email,
		Password: password,
		IMAPHost: GmailIMAPHost,
		IMAPPort: IMAPPort,
		SMTPHost: GmailSMTPHost,
		SMTPPort: SMTPPort,
		Provider: ProviderGmail,
	}
}

func YahooCredentials(email, password string) Credentials {
	return Credentials{
		Email:    email,
		Password: password,
		IMAPHost: YahooIMAPHost,
		IMAPPort: IMAPPort,
		SMTPHost: YahooSMTPHost,
		SMTPPort: SMTPPort,
		Provider: ProviderYahoo,
	}
}

func QQCredentials(email, password string) Credentials {
	return Credentials{
		Email:    email,
		Password: password,
		IMAPHost: QQIMAPHost,
		IMAPPort: IMAPPort,
		SMTPHost: QQSMTPHost,
		SMTPPort: QQSMTPPort,
		Provider: ProviderQQ,
	}
}

func LoadAccountStore() (*AccountStore, error) {
	configDir, err := getConfigDir()
	if err != nil {
		return nil, err
	}

	storePath := filepath.Join(configDir, accountsFileName)
	data, err := os.ReadFile(storePath)
	if err != nil {
		if os.IsNotExist(err) {
			return &AccountStore{}, nil
		}
		return nil, err
	}

	var store AccountStore
	if err := yaml.Unmarshal(data, &store); err != nil {
		return nil, err
	}

	return &store, nil
}

func (s *AccountStore) Save() error {
	configDir, err := getConfigDir()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(configDir, 0700); err != nil {
		return err
	}

	storePath := filepath.Join(configDir, accountsFileName)
	data, err := yaml.Marshal(s)
	if err != nil {
		return err
	}

	return os.WriteFile(storePath, data, 0600)
}

func (s *AccountStore) AddAccount(account Account) {
	// Check if account with same email exists
	for i, a := range s.Accounts {
		if a.Credentials.Email == account.Credentials.Email {
			s.Accounts[i] = account
			return
		}
	}
	s.Accounts = append(s.Accounts, account)
}

func (s *AccountStore) RemoveAccount(email string) bool {
	for i, a := range s.Accounts {
		if a.Credentials.Email == email {
			s.Accounts = append(s.Accounts[:i], s.Accounts[i+1:]...)
			return true
		}
	}
	return false
}

func (s *AccountStore) GetAccount(email string) *Account {
	for _, a := range s.Accounts {
		if a.Credentials.Email == email {
			return &a
		}
	}
	return nil
}

func PromptGmailCredentials() (*Account, error) {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println()
	fmt.Println("  Gmail Login")
	fmt.Println("  ───────────")
	fmt.Println()
	fmt.Println("  You need an App Password to continue.")
	fmt.Println()
	fmt.Println("  1. Enable 2-Step Verification (if not done)")
	fmt.Println("  2. Go to: https://myaccount.google.com/apppasswords")
	fmt.Println("  3. Create an app password for 'Mail'")
	fmt.Println()

	fmt.Print("  Email: ")
	email, err := reader.ReadString('\n')
	if err != nil {
		return nil, err
	}
	email = strings.TrimSpace(email)

	fmt.Print("  App Password: ")
	passwordBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
	if err != nil {
		return nil, err
	}
	fmt.Println()

	password := string(passwordBytes)
	var cleaned strings.Builder
	for _, r := range password {
		if !unicode.IsSpace(r) {
			cleaned.WriteRune(r)
		}
	}
	password = cleaned.String()

	creds := GmailCredentials(email, password)
	account := &Account{
		Name:        email,
		Provider:    "gmail",
		Credentials: creds,
	}
	return account, nil
}

func getConfigDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, ".config", "maily"), nil
}
