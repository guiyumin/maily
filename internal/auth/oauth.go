package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/gmail/v1"
)

const (
	credentialsFile = "credentials.json"
	tokenFile       = "token.json"
)

type Auth struct {
	config    *oauth2.Config
	tokenPath string
}

func New() (*Auth, error) {
	configDir, err := getConfigDir()
	if err != nil {
		return nil, err
	}

	credPath := filepath.Join(configDir, credentialsFile)
	b, err := os.ReadFile(credPath)
	if err != nil {
		return nil, fmt.Errorf("unable to read credentials file: %v\nPlease download credentials.json from Google Cloud Console and place it in %s", err, configDir)
	}

	config, err := google.ConfigFromJSON(b, gmail.GmailReadonlyScope, gmail.GmailSendScope, gmail.GmailModifyScope)
	if err != nil {
		return nil, fmt.Errorf("unable to parse credentials: %v", err)
	}

	return &Auth{
		config:    config,
		tokenPath: filepath.Join(configDir, tokenFile),
	}, nil
}

func (a *Auth) GetClient(ctx context.Context) (*http.Client, error) {
	tok, err := a.loadToken()
	if err != nil {
		tok, err = a.getTokenFromWeb(ctx)
		if err != nil {
			return nil, err
		}
		if err := a.saveToken(tok); err != nil {
			return nil, err
		}
	}
	return a.config.Client(ctx, tok), nil
}

func (a *Auth) loadToken() (*oauth2.Token, error) {
	f, err := os.Open(a.tokenPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	tok := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(tok)
	return tok, err
}

func (a *Auth) getTokenFromWeb(ctx context.Context) (*oauth2.Token, error) {
	authURL := a.config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Go to the following link in your browser:\n%v\n\nEnter authorization code: ", authURL)

	var authCode string
	if _, err := fmt.Scan(&authCode); err != nil {
		return nil, fmt.Errorf("unable to read authorization code: %v", err)
	}

	tok, err := a.config.Exchange(ctx, authCode)
	if err != nil {
		return nil, fmt.Errorf("unable to exchange authorization code: %v", err)
	}
	return tok, nil
}

func (a *Auth) saveToken(token *oauth2.Token) error {
	if err := os.MkdirAll(filepath.Dir(a.tokenPath), 0700); err != nil {
		return err
	}

	f, err := os.OpenFile(a.tokenPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("unable to save token: %v", err)
	}
	defer f.Close()

	return json.NewEncoder(f).Encode(token)
}

func getConfigDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	configDir := filepath.Join(homeDir, ".config", "cocomail")
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return "", err
	}
	return configDir, nil
}

func GetConfigDir() (string, error) {
	return getConfigDir()
}
