package ui

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"maily/internal/auth"
	"maily/internal/gmail"
)

type loginState int

const (
	loginStateInput loginState = iota
	loginStateVerifying
	loginStateSuccess
	loginStateError
)

type loginField int

const (
	fieldEmail loginField = iota
	fieldPassword
)

type LoginApp struct {
	provider     string
	emailInput   textinput.Model
	passwordInput textinput.Model
	focusedField loginField
	state        loginState
	spinner      spinner.Model
	width        int
	height       int
	err          error
	account      *auth.Account
}

type verifySuccessMsg struct {
	account *auth.Account
}

type verifyErrorMsg struct {
	err error
}

func NewLoginApp(provider string) LoginApp {
	emailInput := textinput.New()
	emailInput.Placeholder = "you@gmail.com"
	emailInput.Focus()
	emailInput.CharLimit = 100
	emailInput.Width = 40

	passwordInput := textinput.New()
	passwordInput.Placeholder = "App Password"
	passwordInput.EchoMode = textinput.EchoPassword
	passwordInput.EchoCharacter = '•'
	passwordInput.CharLimit = 100
	passwordInput.Width = 40

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = SpinnerStyle

	return LoginApp{
		provider:      provider,
		emailInput:    emailInput,
		passwordInput: passwordInput,
		focusedField:  fieldEmail,
		state:         loginStateInput,
		spinner:       s,
	}
}

func (a LoginApp) Init() tea.Cmd {
	return textinput.Blink
}

func (a LoginApp) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch a.state {
		case loginStateInput:
			switch msg.String() {
			case "ctrl+c", "esc":
				return a, tea.Quit

			case "tab":
				// Circular tab behavior
				if a.focusedField == fieldEmail {
					a.focusedField = fieldPassword
					a.emailInput.Blur()
					a.passwordInput.Focus()
				} else {
					a.focusedField = fieldEmail
					a.passwordInput.Blur()
					a.emailInput.Focus()
				}

			case "up":
				if a.focusedField == fieldPassword {
					a.focusedField = fieldEmail
					a.passwordInput.Blur()
					a.emailInput.Focus()
				}

			case "down":
				if a.focusedField == fieldEmail {
					a.focusedField = fieldPassword
					a.emailInput.Blur()
					a.passwordInput.Focus()
				}

			case "enter":
				if a.focusedField == fieldEmail {
					a.focusedField = fieldPassword
					a.emailInput.Blur()
					a.passwordInput.Focus()
				} else if a.focusedField == fieldPassword {
					if a.emailInput.Value() != "" && a.passwordInput.Value() != "" {
						a.state = loginStateVerifying
						return a, tea.Batch(a.spinner.Tick, a.verifyCredentials())
					}
				}
			}

		case loginStateSuccess, loginStateError:
			if msg.String() == "enter" || msg.String() == "q" || msg.String() == "esc" {
				return a, tea.Quit
			}
		}

	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height

	case spinner.TickMsg:
		var cmd tea.Cmd
		a.spinner, cmd = a.spinner.Update(msg)
		cmds = append(cmds, cmd)

	case verifySuccessMsg:
		a.state = loginStateSuccess
		a.account = msg.account
		return a, tea.Quit // Auto-transition to email list

	case verifyErrorMsg:
		a.state = loginStateError
		a.err = msg.err
	}

	// Update text inputs
	if a.state == loginStateInput {
		var cmd tea.Cmd
		if a.focusedField == fieldEmail {
			a.emailInput, cmd = a.emailInput.Update(msg)
			cmds = append(cmds, cmd)
		} else {
			a.passwordInput, cmd = a.passwordInput.Update(msg)
			cmds = append(cmds, cmd)
		}
	}

	return a, tea.Batch(cmds...)
}

func (a LoginApp) verifyCredentials() tea.Cmd {
	email := a.emailInput.Value()
	password := a.passwordInput.Value()

	// Clean password (remove all whitespace)
	var cleaned strings.Builder
	for _, r := range password {
		if !unicode.IsSpace(r) {
			cleaned.WriteRune(r)
		}
	}
	password = cleaned.String()

	return func() tea.Msg {
		creds := auth.GmailCredentials(email, password)
		account := &auth.Account{
			Name:        email,
			Provider:    "gmail",
			Credentials: creds,
		}

		// Test connection
		client, err := gmail.NewIMAPClient(&creds)
		if err != nil {
			return verifyErrorMsg{err: err}
		}
		client.Close()

		// Save to account store
		store, err := auth.LoadAccountStore()
		if err != nil {
			return verifyErrorMsg{err: err}
		}

		store.AddAccount(*account)
		if err := store.Save(); err != nil {
			return verifyErrorMsg{err: err}
		}

		return verifySuccessMsg{account: account}
	}
}

func (a LoginApp) View() string {
	if a.width == 0 {
		return "Loading..."
	}

	var content string

	switch a.state {
	case loginStateInput:
		content = a.renderInputForm()

	case loginStateVerifying:
		content = lipgloss.Place(
			a.width,
			a.height-2,
			lipgloss.Center,
			lipgloss.Center,
			fmt.Sprintf("%s Verifying credentials...", a.spinner.View()),
		)

	case loginStateSuccess:
		successMsg := lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#10B981")).
			Render(fmt.Sprintf("✓ Logged in as %s", a.account.Credentials.Email))

		hint := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#9CA3AF")).
			Render("\n\nRun 'maily' to start.\n\nPress Enter to exit.")

		content = lipgloss.Place(
			a.width,
			a.height-2,
			lipgloss.Center,
			lipgloss.Center,
			successMsg+hint,
		)

	case loginStateError:
		errorMsg := lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#EF4444")).
			Render(fmt.Sprintf("✗ Login failed: %v", a.err))

		hint := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#9CA3AF")).
			Render("\n\nMake sure you:\n• Used an App Password (not your regular password)\n• Have IMAP enabled in Gmail settings\n\nPress Enter to exit.")

		content = lipgloss.Place(
			a.width,
			a.height-2,
			lipgloss.Center,
			lipgloss.Center,
			errorMsg+hint,
		)
	}

	return content
}

func (a LoginApp) renderInputForm() string {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#7C3AED")).
		MarginBottom(1)

	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#F9FAFB")).
		Width(12)

	focusedLabelStyle := labelStyle.Copy().
		Bold(true).
		Foreground(lipgloss.Color("#7C3AED"))

	hintStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9CA3AF"))

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#7C3AED")).
		Padding(1, 3)

	// Title
	title := titleStyle.Render("Gmail Login")

	// Instructions
	instructions := hintStyle.Render("You need an App Password to continue.\n\n1. Enable 2-Step Verification (if not done)\n2. Go to: myaccount.google.com/apppasswords\n3. Type a name and click Create\n")

	// Email field
	emailLabel := labelStyle
	if a.focusedField == fieldEmail {
		emailLabel = focusedLabelStyle
	}
	emailRow := lipgloss.JoinHorizontal(
		lipgloss.Left,
		emailLabel.Render("Email:"),
		a.emailInput.View(),
	)

	// Password field
	passwordLabel := labelStyle
	if a.focusedField == fieldPassword {
		passwordLabel = focusedLabelStyle
	}
	passwordRow := lipgloss.JoinHorizontal(
		lipgloss.Left,
		passwordLabel.Render("App Password:"),
		a.passwordInput.View(),
	)

	// Hint
	hint := hintStyle.Render("\nTab to switch fields • Enter to submit • Esc to cancel")

	form := lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		"",
		instructions,
		"",
		emailRow,
		"",
		passwordRow,
		"",
		hint,
	)

	return lipgloss.Place(
		a.width,
		a.height-2,
		lipgloss.Center,
		lipgloss.Center,
		boxStyle.Render(form),
	)
}

// GetAccount returns the logged in account (for use after TUI exits)
func (a LoginApp) GetAccount() *auth.Account {
	return a.account
}

// Success returns whether login was successful
func (a LoginApp) Success() bool {
	return a.state == loginStateSuccess
}
