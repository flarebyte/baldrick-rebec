package conversation

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	cfgpkg "github.com/flarebyte/baldrick-rebec/internal/config"
	pgdao "github.com/flarebyte/baldrick-rebec/internal/dao/postgres"
	"github.com/spf13/cobra"
)

var (
	flagConvActiveRole string
)

// UI styles (reused palette)
var (
	cStyleHeader  = lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Bold(true)  // cyan
	cStyleLabel   = lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Bold(true)   // gray
	cStyleValue   = lipgloss.NewStyle().Foreground(lipgloss.Color("7"))              // white-ish
	cStyleHelp    = lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Italic(true) // dim italic
	cStyleCursor  = lipgloss.NewStyle().Foreground(lipgloss.Color("13")).Bold(true)  // magenta
	cStyleDivider = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))              // gray line
)

var activeCmd = &cobra.Command{
	Use:   "active",
	Short: "Interactive list of recent conversations for a role",
	RunE: func(cmd *cobra.Command, args []string) error {
		role := strings.TrimSpace(flagConvActiveRole)
		if role == "" {
			return errors.New("--role is required")
		}
		cfg, err := cfgpkg.Load()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		db, err := pgdao.OpenApp(ctx, cfg)
		if err != nil {
			return err
		}
		defer db.Close()

		// Load roles for cycling (limit to a reasonable number)
		roles, err := pgdao.ListRoles(ctx, db, 100, 0)
		if err != nil {
			return err
		}
		roleNames := make([]string, 0, len(roles))
		for _, r := range roles {
			roleNames = append(roleNames, r.Name)
		}
		// Ensure current role exists in the cycle list
		if idxOf(roleNames, role) < 0 {
			roleNames = append([]string{role}, roleNames...)
		}

		// Fetch most recent conversations for current role (max 10)
		convs, err := pgdao.ListConversations(ctx, db, "", role, 10, 0)
		if err != nil {
			return err
		}

		fmt.Fprintf(os.Stderr, "conversation active: role=%s conversations=%d roles=%d\n", role, len(convs), len(roleNames))
		m := newConvActiveModel(roleNames, role, convs)
		if _, err := tea.NewProgram(m).Run(); err != nil {
			return err
		}
		return nil
	},
}

func init() {
	ConversationCmd.AddCommand(activeCmd)
	activeCmd.Flags().StringVar(&flagConvActiveRole, "role", "", "Role name (required)")
}

// Model
type convActiveModel struct {
	roles         []string
	roleIdx       int
	conversations []pgdao.Conversation
	cursor        int
	quitting      bool
	err           string
}

func newConvActiveModel(roles []string, currentRole string, conversations []pgdao.Conversation) convActiveModel {
	idx := idxOf(roles, currentRole)
	if idx < 0 {
		idx = 0
	}
	return convActiveModel{roles: roles, roleIdx: idx, conversations: conversations}
}

func (m convActiveModel) Init() tea.Cmd { return nil }

func (m convActiveModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.quitting = true
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.conversations)-1 {
				m.cursor++
			}
		case "n":
			// Next role (cycle)
			if len(m.roles) > 0 {
				m.err = ""
				m.roleIdx = (m.roleIdx + 1) % len(m.roles)
				m.cursor = 0
				return m, refreshConversationsCmd(m.roles[m.roleIdx])
			}
		case "r":
			// Refresh current role
			m.err = ""
			return m, refreshConversationsCmd(m.roles[m.roleIdx])
		}
	case convRefreshMsg:
		m.conversations = msg.conversations
		if m.cursor >= len(m.conversations) {
			m.cursor = len(m.conversations) - 1
			if m.cursor < 0 {
				m.cursor = 0
			}
		}
		return m, nil
	case convErrMsg:
		m.err = msg.err.Error()
		return m, nil
	}
	return m, nil
}

func (m convActiveModel) View() string {
	var b strings.Builder
	// Header
	b.WriteString(cStyleHeader.Render("Conversations Viewer") + "\n")
	currentRole := ""
	if len(m.roles) > 0 {
		currentRole = m.roles[m.roleIdx]
	}
	b.WriteString(cStyleLabel.Render("Role: ") + cStyleValue.Render(fmt.Sprintf("[%d/%d] %s", m.roleIdx+1, len(m.roles), currentRole)) + "\n")

	// Divider
	b.WriteString(cStyleDivider.Render(strings.Repeat("─", 60)) + "\n")
	// Help
	b.WriteString(cStyleHelp.Render("Keys: ↑/k, ↓/j, n=next role, r=refresh, enter, q") + "\n")

	if len(m.conversations) == 0 {
		b.WriteString("No conversations.\n")
		return b.String()
	}

	// List
	for i, c := range m.conversations {
		cursor := "  "
		if i == m.cursor {
			cursor = cStyleCursor.Render("> ")
		}
		title := c.Title
		if strings.TrimSpace(title) == "" {
			title = "(untitled)"
		}
		fmt.Fprintf(&b, "%s%s\n", cursor, cStyleValue.Render(title))
	}

	if m.err != "" {
		b.WriteString("\n")
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Render("Error: "))
		b.WriteString(m.err)
		b.WriteString("\n")
	}

	// Details
	if m.cursor >= 0 && m.cursor < len(m.conversations) {
		c := m.conversations[m.cursor]
		b.WriteString(cStyleDivider.Render(strings.Repeat("─", 60)) + "\n")
		b.WriteString(cStyleHeader.Render("Details") + "\n")
		b.WriteString(cStyleLabel.Render("ID: ") + cStyleValue.Render(c.ID) + "\n")
		b.WriteString(cStyleLabel.Render("Title: ") + cStyleValue.Render(c.Title) + "\n")
		if c.Project.Valid {
			b.WriteString(cStyleLabel.Render("Project: ") + cStyleValue.Render(c.Project.String) + "\n")
		}
		if c.Description.Valid {
			b.WriteString(cStyleLabel.Render("Description: ") + cStyleValue.Render(c.Description.String) + "\n")
		}
		if c.Notes.Valid {
			b.WriteString(cStyleLabel.Render("Notes: ") + cStyleValue.Render(c.Notes.String) + "\n")
		}
		if c.Created.Valid {
			b.WriteString(cStyleLabel.Render("Created: ") + cStyleValue.Render(c.Created.Time.Format(time.RFC3339)) + "\n")
		}
		if c.Updated.Valid {
			b.WriteString(cStyleLabel.Render("Updated: ") + cStyleValue.Render(c.Updated.Time.Format(time.RFC3339)) + "\n")
		}
	}
	return b.String()
}

// Messages + commands
type convRefreshMsg struct{ conversations []pgdao.Conversation }
type convErrMsg struct{ err error }

func refreshConversationsCmd(role string) tea.Cmd {
	return func() tea.Msg {
		cfg, err := cfgpkg.Load()
		if err != nil {
			return convErrMsg{err}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		db, err := pgdao.OpenApp(ctx, cfg)
		if err != nil {
			return convErrMsg{err}
		}
		defer db.Close()
		rows, err := pgdao.ListConversations(ctx, db, "", role, 10, 0)
		if err != nil {
			return convErrMsg{err}
		}
		return convRefreshMsg{conversations: rows}
	}
}

// util
func idxOf(arr []string, needle string) int {
	for i, v := range arr {
		if v == needle {
			return i
		}
	}
	return -1
}
