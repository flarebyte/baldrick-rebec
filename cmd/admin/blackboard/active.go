package blackboard

import (
	"context"
	"database/sql"
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

// UI styles
var (
	bStyleHeader  = lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Bold(true)  // cyan
	bStyleLabel   = lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Bold(true)   // gray
	bStyleValue   = lipgloss.NewStyle().Foreground(lipgloss.Color("7"))              // white-ish
	bStyleHelp    = lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Italic(true) // dim italic
	bStyleCursor  = lipgloss.NewStyle().Foreground(lipgloss.Color("13")).Bold(true)  // magenta
	bStyleDivider = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))              // gray line
	// Related entity labels (distinct colors)
	bStyleStoreLabel = lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true)
	bStyleTaskLabel  = lipgloss.NewStyle().Foreground(lipgloss.Color("14")).Bold(true)
	bStyleConvLabel  = lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Bold(true)
	bStyleProjLabel  = lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Bold(true)
)

var activeCmd = &cobra.Command{
	Use:   "active",
	Short: "Interactive list of recent blackboards per role",
	RunE: func(cmd *cobra.Command, args []string) error {
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

		// Load roles for cycling
		roles, err := pgdao.ListRoles(ctx, db, 100, 0)
		if err != nil {
			return err
		}
		roleNames := make([]string, 0, len(roles))
		for _, r := range roles {
			roleNames = append(roleNames, r.Name)
		}
		if len(roleNames) == 0 {
			return fmt.Errorf("no roles found")
		}
		role := roleNames[0]

		// Fetch most recent blackboards for current role (max 10)
		boards, err := pgdao.ListBlackboardsWithRefs(ctx, db, role, 10, 0)
		if err != nil {
			return err
		}

		fmt.Fprintf(os.Stderr, "blackboard active: role=%s boards=%d roles=%d\n", role, len(boards), len(roleNames))
		m := newBBActiveModel(roleNames, role, boards)
		if _, err := tea.NewProgram(m).Run(); err != nil {
			return err
		}
		return nil
	},
}

func init() { BlackboardCmd.AddCommand(activeCmd) }

// Model
type bbActiveModel struct {
	roles    []string
	roleIdx  int
	boards   []pgdao.BlackboardWithRefs
	cursor   int
	quitting bool
	err      string
}

func newBBActiveModel(roles []string, currentRole string, boards []pgdao.BlackboardWithRefs) bbActiveModel {
	idx := -1
	for i, r := range roles {
		if r == currentRole {
			idx = i
			break
		}
	}
	if idx < 0 {
		idx = 0
	}
	return bbActiveModel{roles: roles, roleIdx: idx, boards: boards}
}

func (m bbActiveModel) Init() tea.Cmd { return nil }

func (m bbActiveModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
			if m.cursor < len(m.boards)-1 {
				m.cursor++
			}
		case "n":
			// Next role (cycle)
			if len(m.roles) > 0 {
				m.err = ""
				m.roleIdx = (m.roleIdx + 1) % len(m.roles)
				m.cursor = 0
				return m, refreshBoardsCmd(m.roles[m.roleIdx])
			}
		case "r":
			// Refresh current role
			m.err = ""
			return m, refreshBoardsCmd(m.roles[m.roleIdx])
		}
	case bbRefreshMsg:
		m.boards = msg.boards
		if m.cursor >= len(m.boards) {
			m.cursor = len(m.boards) - 1
			if m.cursor < 0 {
				m.cursor = 0
			}
		}
		return m, nil
	case bbErrMsg:
		m.err = msg.err.Error()
		return m, nil
	}
	return m, nil
}

func (m bbActiveModel) View() string {
	var b strings.Builder
	// Header
	b.WriteString(bStyleHeader.Render("Blackboards Viewer") + "\n")
	currentRole := ""
	if len(m.roles) > 0 {
		currentRole = m.roles[m.roleIdx]
	}
	b.WriteString(bStyleLabel.Render("Role: ") + bStyleValue.Render(fmt.Sprintf("[%d/%d] %s", m.roleIdx+1, len(m.roles), currentRole)) + "\n")

	// Divider and help
	b.WriteString(bStyleDivider.Render(strings.Repeat("─", 60)) + "\n")
	b.WriteString(bStyleHelp.Render("Keys: ↑/k, ↓/j, n=next role, r=refresh, enter, q") + "\n")

	if len(m.boards) == 0 {
		b.WriteString("No blackboards.\n")
		return b.String()
	}

	// List rows: show a concise label (project or conversation title, else store)
	for i, bb := range m.boards {
		cursor := "  "
		if i == m.cursor {
			cursor = bStyleCursor.Render("> ")
		}
		primary := firstNonEmpty(strOrNull(bb.ProjectName), strOrNull(bb.ConversationTitle), strOrNull(bb.StoreTitle), bb.StoreID)
		if primary == "" {
			primary = bb.ID
		}
		chips := relatedChips(bb)
		if chips != "" {
			fmt.Fprintf(&b, "%s%s %s\n", cursor, bStyleValue.Render(primary), chips)
		} else {
			fmt.Fprintf(&b, "%s%s\n", cursor, bStyleValue.Render(primary))
		}
	}

	if m.err != "" {
		b.WriteString("\n")
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Render("Error: "))
		b.WriteString(m.err)
		b.WriteString("\n")
	}

	// Details for selected board
	if m.cursor >= 0 && m.cursor < len(m.boards) {
		bb := m.boards[m.cursor]
		b.WriteString(bStyleDivider.Render(strings.Repeat("─", 60)) + "\n")
		b.WriteString(bStyleHeader.Render("Details") + "\n")
		b.WriteString(bStyleLabel.Render("ID: ") + bStyleValue.Render(bb.ID) + "\n")
		// Store (distinct style)
		b.WriteString(bStyleStoreLabel.Render("Store: ") + bStyleValue.Render(bb.StoreID) + "\n")
		if bb.StoreName.Valid {
			b.WriteString(bStyleStoreLabel.Render("Store.name: ") + bStyleValue.Render(bb.StoreName.String) + "\n")
		}
		if bb.StoreTitle.Valid {
			b.WriteString(bStyleStoreLabel.Render("Store.title: ") + bStyleValue.Render(bb.StoreTitle.String) + "\n")
		}
		// Role
		b.WriteString(bStyleLabel.Render("Role: ") + bStyleValue.Render(bb.RoleName) + "\n")
		// Project (distinct style)
		if bb.ProjectName.Valid {
			b.WriteString(bStyleProjLabel.Render("Project: ") + bStyleValue.Render(bb.ProjectName.String) + "\n")
		}
		if bb.ProjectDesc.Valid {
			b.WriteString(bStyleProjLabel.Render("Project.desc: ") + bStyleValue.Render(bb.ProjectDesc.String) + "\n")
		}
		// Conversation (distinct style)
		if bb.ConversationID.Valid {
			b.WriteString(bStyleConvLabel.Render("Conversation: ") + bStyleValue.Render(bb.ConversationID.String) + "\n")
		}
		if bb.ConversationTitle.Valid {
			b.WriteString(bStyleConvLabel.Render("Conversation.title: ") + bStyleValue.Render(bb.ConversationTitle.String) + "\n")
		}
		// Task (distinct style)
		if bb.TaskID.Valid {
			b.WriteString(bStyleTaskLabel.Render("Task: ") + bStyleValue.Render(bb.TaskID.String) + "\n")
		}
		if bb.TaskVariant.Valid {
			b.WriteString(bStyleTaskLabel.Render("Task.variant: ") + bStyleValue.Render(bb.TaskVariant.String) + "\n")
		}
		if bb.TaskTitle.Valid {
			b.WriteString(bStyleTaskLabel.Render("Task.title: ") + bStyleValue.Render(bb.TaskTitle.String) + "\n")
		}
		if bb.Background.Valid {
			b.WriteString(bStyleLabel.Render("Background: ") + bStyleValue.Render(bb.Background.String) + "\n")
		}
		if bb.Guidelines.Valid {
			b.WriteString(bStyleLabel.Render("Guidelines: ") + bStyleValue.Render(bb.Guidelines.String) + "\n")
		}
		if bb.Created.Valid {
			b.WriteString(bStyleLabel.Render("Created: ") + bStyleValue.Render(bb.Created.Time.Format(time.RFC3339)) + "\n")
		}
		if bb.Updated.Valid {
			b.WriteString(bStyleLabel.Render("Updated: ") + bStyleValue.Render(bb.Updated.Time.Format(time.RFC3339)) + "\n")
		}
	}
	return b.String()
}

// Messages + commands
type bbRefreshMsg struct{ boards []pgdao.BlackboardWithRefs }
type bbErrMsg struct{ err error }

func refreshBoardsCmd(role string) tea.Cmd {
	return func() tea.Msg {
		cfg, err := cfgpkg.Load()
		if err != nil {
			return bbErrMsg{err}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		db, err := pgdao.OpenApp(ctx, cfg)
		if err != nil {
			return bbErrMsg{err}
		}
		defer db.Close()
		rows, err := pgdao.ListBlackboardsWithRefs(ctx, db, role, 10, 0)
		if err != nil {
			return bbErrMsg{err}
		}
		return bbRefreshMsg{boards: rows}
	}
}

// helpers
func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
func strOrNull(ns sql.NullString) string {
	if ns.Valid {
		return ns.String
	}
	return ""
}

// relatedChips builds a small inline string with joined fields, styled to stand out.
func relatedChips(bb pgdao.BlackboardWithRefs) string {
	var parts []string
	if bb.StoreName.Valid {
		parts = append(parts, lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Render("store:"+bb.StoreName.String))
	}
	if bb.TaskVariant.Valid || bb.TaskTitle.Valid {
		label := strOrNull(bb.TaskVariant)
		if bb.TaskTitle.Valid && strings.TrimSpace(bb.TaskTitle.String) != "" {
			if label != "" {
				label += " · "
			}
			label += bb.TaskTitle.String
		}
		if label != "" {
			parts = append(parts, lipgloss.NewStyle().Foreground(lipgloss.Color("14")).Render("task:"+label))
		}
	}
	if bb.ConversationTitle.Valid {
		parts = append(parts, lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Render("conv:"+bb.ConversationTitle.String))
	}
	if bb.ProjectName.Valid {
		parts = append(parts, lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Render("proj:"+bb.ProjectName.String))
	}
	if len(parts) == 0 {
		return ""
	}
	return "[" + strings.Join(parts, "  ") + "]"
}
