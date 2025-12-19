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

type Credentials struct {
	Email    string `yaml:"email"`
	Password string `yaml:"password"`
	IMAPHost string `yaml:"imap_host"`
	IMAPPort int    `yaml:"imap_port"`
	SMTPHost string `yaml:"smtp_host"`
	SMTPPort int    `yaml:"smtp_port"`
}

type Account struct {
	Name        string      `yaml:"name"`
	Provider    string      `yaml:"provider"`
	Credentials Credentials `yaml:"credentials"`
}

type AccountStore struct {
	Active   string    `yaml:"active"`
	Accounts []Account `yaml:"accounts"`
}

func GmailCredentials(email, password string) Credentials {
	return Credentials{
		Email:    email,
		Password: password,
		IMAPHost: "imap.gmail.com",
		IMAPPort: 993,
		SMTPHost: "smtp.gmail.com",
		SMTPPort: 587,
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
			if s.Active == email {
				s.Active = ""
			}
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

func (s *AccountStore) GetActiveAccount() *Account {
	if s.Active == "" && len(s.Accounts) > 0 {
		return &s.Accounts[0]
	}
	return s.GetAccount(s.Active)
}

func (s *AccountStore) SetActive(email string) bool {
	for _, a := range s.Accounts {
		if a.Credentials.Email == email {
			s.Active = email
			return true
		}
	}
	return false
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
	fmt.Println("  2. Go to: myaccount.google.com/apppasswords")
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
	return filepath.Join(homeDir, ".config", "cocomail"), nil
}
