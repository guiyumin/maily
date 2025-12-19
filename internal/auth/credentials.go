package auth

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/term"
)

const credentialsFileName = "credentials.json"

type Credentials struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	IMAPHost string `json:"imap_host"`
	IMAPPort int    `json:"imap_port"`
	SMTPHost string `json:"smtp_host"`
	SMTPPort int    `json:"smtp_port"`
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

func LoadCredentials() (*Credentials, error) {
	configDir, err := getConfigDir()
	if err != nil {
		return nil, err
	}

	credPath := filepath.Join(configDir, credentialsFileName)
	data, err := os.ReadFile(credPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var creds Credentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil, err
	}

	return &creds, nil
}

func SaveCredentials(creds *Credentials) error {
	configDir, err := getConfigDir()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(configDir, 0700); err != nil {
		return err
	}

	credPath := filepath.Join(configDir, credentialsFileName)
	data, err := json.MarshalIndent(creds, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(credPath, data, 0600)
}

func PromptCredentials() (*Credentials, error) {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println("┌─────────────────────────────────────────────────┐")
	fmt.Println("│              COCOMAIL SETUP                     │")
	fmt.Println("├─────────────────────────────────────────────────┤")
	fmt.Println("│ To use Gmail, you need an App Password:        │")
	fmt.Println("│ 1. Enable 2-Step Verification (if not done)    │")
	fmt.Println("│ 2. Go to: myaccount.google.com/apppasswords    │")
	fmt.Println("│ 3. Generate a password for 'Mail'              │")
	fmt.Println("└─────────────────────────────────────────────────┘")
	fmt.Println()

	fmt.Print("Email: ")
	email, err := reader.ReadString('\n')
	if err != nil {
		return nil, err
	}
	email = strings.TrimSpace(email)

	fmt.Print("App Password: ")
	passwordBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
	if err != nil {
		return nil, err
	}
	fmt.Println()

	password := strings.TrimSpace(string(passwordBytes))
	password = strings.ReplaceAll(password, " ", "")

	creds := GmailCredentials(email, password)
	return &creds, nil
}

func DeleteCredentials() error {
	configDir, err := getConfigDir()
	if err != nil {
		return err
	}

	credPath := filepath.Join(configDir, credentialsFileName)
	return os.Remove(credPath)
}

func getConfigDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, ".config", "cocomail"), nil
}
