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
	"maily/internal/i18n"
	"maily/internal/mail"
	"maily/internal/ui/components"
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
	emailInput.Focus()
	emailInput.CharLimit = 100
	emailInput.Width = 40

	// Set placeholder based on provider
	switch provider {
	case "yahoo":
		emailInput.Placeholder = "you@yahoo.com"
	case "qq":
		emailInput.Placeholder = "you@qq.com"
	default:
		emailInput.Placeholder = "you@mail.com"
	}

	passwordInput := textinput.New()
	if provider == "qq" {
		passwordInput.Placeholder = "Authorization Code"
	} else {
		passwordInput.Placeholder = "App Password"
	}
	passwordInput.EchoMode = textinput.EchoPassword
	passwordInput.EchoCharacter = '•'
	passwordInput.CharLimit = 100
	passwordInput.Width = 40

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = components.SpinnerStyle

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
				switch a.focusedField {
				case fieldEmail:
					a.focusedField = fieldPassword
					a.emailInput.Blur()
					a.passwordInput.Focus()
				case fieldPassword:
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
	provider := a.provider

	// Clean password (remove all whitespace)
	var cleaned strings.Builder
	for _, r := range password {
		if !unicode.IsSpace(r) {
			cleaned.WriteRune(r)
		}
	}
	password = cleaned.String()

	return func() tea.Msg {
		var creds auth.Credentials
		switch provider {
		case "yahoo":
			creds = auth.YahooCredentials(email, password)
		case "qq":
			creds = auth.QQCredentials(email, password)
		default:
			creds = auth.GmailCredentials(email, password)
		}

		account := &auth.Account{
			Name:        email,
			Provider:    provider,
			Credentials: creds,
		}

		// Test connection
		client, err := mail.NewIMAPClient(&creds)
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

		var hintText string
		switch a.provider {
		case "qq":
			hintText = "\n\n" + i18n.T("login.qq.error_hint") + "\n\n" + i18n.T("login.hint_exit")
		default:
			hintText = "\n\n" + i18n.T("error.auth_hint") + "\n\n" + i18n.T("login.hint_exit")
		}

		hint := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#9CA3AF")).
			Render(hintText)

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

	focusedLabelStyle := labelStyle.
		Bold(true).
		Foreground(lipgloss.Color("#7C3AED"))

	hintStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9CA3AF"))

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#7C3AED")).
		Padding(1, 3)

	// Title and instructions based on provider
	var title, instructions string
	switch a.provider {
	case "yahoo":
		title = titleStyle.Render(i18n.T("login.yahoo.title"))
		instructions = hintStyle.Render(i18n.T("login.yahoo.hint"))
	case "qq":
		title = titleStyle.Render(i18n.T("login.qq.title"))
		instructions = hintStyle.Render(i18n.T("login.qq.hint"))
	default:
		title = titleStyle.Render(i18n.T("login.gmail.title"))
		instructions = hintStyle.Render(i18n.T("login.gmail.hint"))
	}

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
	passwordLabelText := i18n.T("login.password_label")
	if a.provider == "qq" {
		passwordLabelText = i18n.T("login.qq.password_label")
	}
	passwordRow := lipgloss.JoinHorizontal(
		lipgloss.Left,
		passwordLabel.Render(passwordLabelText),
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
