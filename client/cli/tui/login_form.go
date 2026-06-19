// Package tui provides interactive terminal UI components built with Bubbletea.
package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ── Constants ────────────────────────────────────────────────────────────────

const (
	fieldEmail = iota
	fieldPassword
)

// ── Colors ───────────────────────────────────────────────────────────────────

var (
	colorPrimary = lipgloss.Color("#5B9BD5")
	colorAccent  = lipgloss.Color("#4EC9B0")
	colorSuccess = lipgloss.Color("#6BCB77")
	colorMuted   = lipgloss.Color("#6C6C6C")
	colorError   = lipgloss.Color("#FF6B6B")
)

// ── Styles ───────────────────────────────────────────────────────────────────

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorPrimary).
			Padding(0, 1)

	subtitleStyle = lipgloss.NewStyle().
			Foreground(colorAccent).
			Italic(true)

	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorPrimary).
			Padding(1, 3).
			Width(52)

	labelStyle = lipgloss.NewStyle().
			Foreground(colorAccent).
			Bold(true)

	activeInputStyle = lipgloss.NewStyle().
				Border(lipgloss.NormalBorder(), false, false, true, false).
				BorderForeground(colorPrimary).
				Padding(0, 1)

	inactiveInputStyle = lipgloss.NewStyle().
				Border(lipgloss.NormalBorder(), false, false, true, false).
				BorderForeground(colorMuted).
				Padding(0, 1)

	hintStyle = lipgloss.NewStyle().Foreground(colorMuted).Italic(true)

	errorStyle = lipgloss.NewStyle().Foreground(colorError).Bold(true)

	helpStyle = lipgloss.NewStyle().Foreground(colorMuted)

	submitStyle = lipgloss.NewStyle().
			Foreground(colorSuccess).
			Bold(true).
			Padding(0, 1)
)

// ── Types ────────────────────────────────────────────────────────────────────

// LoginResult holds the values collected by the login form.
// Cancelled is true when the user pressed Ctrl+C or Esc.
type LoginResult struct {
	Email     string
	Password  string
	Cancelled bool
}

type loginModel struct {
	inputs  [2]textinput.Model
	focused int
	err     string
	done    bool
	result  LoginResult
}

// ── Model ────────────────────────────────────────────────────────────────────

func initialLoginModel() loginModel {
	emailInput := textinput.New()
	emailInput.Placeholder = "you@example.com"
	emailInput.CharLimit = 254
	emailInput.Width = 40
	emailInput.Focus()

	passInput := textinput.New()
	passInput.Placeholder = "••••••••"
	passInput.EchoMode = textinput.EchoPassword
	passInput.EchoCharacter = '•'
	passInput.CharLimit = 128
	passInput.Width = 40

	return loginModel{
		inputs:  [2]textinput.Model{emailInput, passInput},
		focused: fieldEmail,
	}
}

func (m loginModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m loginModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			m.result = LoginResult{Cancelled: true}
			m.done = true
			return m, tea.Quit

		case tea.KeyTab, tea.KeyShiftTab, tea.KeyEnter, tea.KeyDown, tea.KeyUp:
			if msg.Type == tea.KeyEnter && m.focused == fieldPassword {
				email := strings.TrimSpace(m.inputs[fieldEmail].Value())
				pass := m.inputs[fieldPassword].Value()
				if email == "" {
					m.err = "Email cannot be empty"
					m.focused = fieldEmail
					m.inputs[fieldEmail].Focus()
					m.inputs[fieldPassword].Blur()
					break
				}
				if pass == "" {
					m.err = "Password cannot be empty"
					break
				}
				m.result = LoginResult{Email: email, Password: pass}
				m.done = true
				return m, tea.Quit
			}

			if m.focused == fieldEmail {
				m.focused = fieldPassword
				m.inputs[fieldEmail].Blur()
				m.inputs[fieldPassword].Focus()
			} else {
				m.focused = fieldEmail
				m.inputs[fieldPassword].Blur()
				m.inputs[fieldEmail].Focus()
			}
			m.err = ""
		}

	case tea.WindowSizeMsg:
	}

	var cmd tea.Cmd
	m.inputs[m.focused], cmd = m.inputs[m.focused].Update(msg)
	return m, cmd
}

func (m loginModel) View() string {
	if m.done {
		return ""
	}

	var sb strings.Builder

	sb.WriteString("\n")
	sb.WriteString(titleStyle.Render("  ☁  Nimbus CLI"))
	sb.WriteString("\n")
	sb.WriteString(subtitleStyle.Render("  Secure Cloud File Storage"))
	sb.WriteString("\n\n")

	emailLabel := labelStyle.Render("  Email")
	var emailInputRendered string
	if m.focused == fieldEmail {
		emailInputRendered = activeInputStyle.Render(m.inputs[fieldEmail].View())
	} else {
		emailInputRendered = inactiveInputStyle.Render(m.inputs[fieldEmail].View())
	}

	passLabel := labelStyle.Render("  Password")
	var passInputRendered string
	if m.focused == fieldPassword {
		passInputRendered = activeInputStyle.Render(m.inputs[fieldPassword].View())
	} else {
		passInputRendered = inactiveInputStyle.Render(m.inputs[fieldPassword].View())
	}

	formContent := fmt.Sprintf(
		"%s\n%s\n\n%s\n%s",
		emailLabel, emailInputRendered,
		passLabel, passInputRendered,
	)

	if m.err != "" {
		formContent += "\n\n" + errorStyle.Render("  ✗ "+m.err)
	}

	if m.focused == fieldPassword {
		formContent += "\n\n" + submitStyle.Render("  ↵  Press Enter to login")
	} else {
		formContent += "\n\n" + hintStyle.Render("  Tab to switch fields")
	}

	sb.WriteString(boxStyle.Render(formContent))
	sb.WriteString("\n")
	sb.WriteString(helpStyle.Render("  Esc / Ctrl+C to quit"))
	sb.WriteString("\n")

	return sb.String()
}

// ── Entry point ──────────────────────────────────────────────────────────────

// RunLoginForm displays the interactive login form and returns the result.
// Callers should check LoginResult.Cancelled before using Email/Password.
func RunLoginForm() (LoginResult, error) {
	p := tea.NewProgram(initialLoginModel(), tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return LoginResult{}, err
	}
	m, ok := finalModel.(loginModel)
	if !ok {
		return LoginResult{}, fmt.Errorf("unexpected model type")
	}
	return m.result, nil
}
