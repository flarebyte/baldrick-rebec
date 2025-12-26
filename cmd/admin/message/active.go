package message

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	cfgpkg "github.com/flarebyte/baldrick-rebec/internal/config"
	pgdao "github.com/flarebyte/baldrick-rebec/internal/dao/postgres"
	"github.com/spf13/cobra"
)

var (
	flagMsgActiveConversation string
)

// UI styles (aligned with testcase/blackboard TUIs)
var (
	mStyleHeader  = lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Bold(true)
	mStyleLabel   = lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Bold(true)
	mStyleValue   = lipgloss.NewStyle().Foreground(lipgloss.Color("7"))
	mStyleHelp    = lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Italic(true)
	mStyleCursor  = lipgloss.NewStyle().Foreground(lipgloss.Color("13")).Bold(true)
	mStyleDivider = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
)

// activeCmd shows messages grouped by experiment for a conversation; cycle experiments with 'n'
var activeCmd = &cobra.Command{
	Use:   "active",
	Short: "Interactive list of messages by experiment for a conversation",
	RunE: func(cmd *cobra.Command, args []string) error {
		if strings.TrimSpace(flagMsgActiveConversation) == "" {
			return errors.New("--conversation is required")
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
		conv, err := pgdao.GetConversationByID(ctx, db, flagMsgActiveConversation)
		if err != nil {
			return err
		}
		role := strings.TrimSpace(conv.RoleName)
		if role == "" {
			return errors.New("conversation has no role_name")
		}
		exps, err := pgdao.ListExperiments(ctx, db, conv.ID, 6, 0)
		if err != nil {
			return err
		}
		if len(exps) == 0 {
			return fmt.Errorf("no experiment found for conversation %s", conv.ID)
		}
		// initial messages for latest experiment
		msgs, err := pgdao.ListMessages(ctx, db, role, exps[0].ID, "", "", 100, 0)
		if err != nil {
			return err
		}
		m := newMsgActiveModel(conv.ID, role, exps, 0, msgs)
		if _, err := tea.NewProgram(m).Run(); err != nil {
			return err
		}
		return nil
	},
}

func init() {
	MessageCmd.AddCommand(activeCmd)
	activeCmd.Flags().StringVar(&flagMsgActiveConversation, "conversation", "", "Conversation ID (required)")
}

// Bubble Tea model
type msgActiveModel struct {
	conversation string
	role         string
	experiments  []pgdao.Experiment
	expIdx       int
	messages     []pgdao.MessageEvent
	cursor       int
	quitting     bool
	err          string
	contentCache map[string]pgdao.ContentRecord // by contentID
}

func newMsgActiveModel(conversation, role string, experiments []pgdao.Experiment, expIdx int, messages []pgdao.MessageEvent) msgActiveModel {
	if expIdx < 0 || expIdx >= len(experiments) {
		expIdx = 0
	}
	return msgActiveModel{conversation: conversation, role: role, experiments: experiments, expIdx: expIdx, messages: messages, contentCache: map[string]pgdao.ContentRecord{}}
}

func (m msgActiveModel) Init() tea.Cmd { return nil }

func (m msgActiveModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
			if len(m.messages) > 0 && m.cursor >= 0 && m.cursor < len(m.messages) {
				me := m.messages[m.cursor]
				if _, ok := m.contentCache[me.ContentID]; !ok {
					return m, fetchContentCmd(me.ContentID)
				}
			}
			return m, nil
		case "down", "j":
			if m.cursor < len(m.messages)-1 {
				m.cursor++
			}
			if len(m.messages) > 0 && m.cursor >= 0 && m.cursor < len(m.messages) {
				me := m.messages[m.cursor]
				if _, ok := m.contentCache[me.ContentID]; !ok {
					return m, fetchContentCmd(me.ContentID)
				}
			}
			return m, nil
		case "r":
			return m, refreshMsgsCmd(m.role, m.experiments[m.expIdx].ID)
		case "n":
			if len(m.experiments) > 0 {
				m.expIdx = (m.expIdx + 1) % len(m.experiments)
				m.cursor = 0
				return m, refreshMsgsCmd(m.role, m.experiments[m.expIdx].ID)
			}
		}
	case msgRefreshMsg:
		m.messages = msg.messages
		// reset cursor safely
		if m.cursor >= len(m.messages) {
			if len(m.messages) == 0 {
				m.cursor = 0
			} else {
				m.cursor = len(m.messages) - 1
			}
		}
		if len(m.messages) > 0 {
			me := m.messages[m.cursor]
			if _, ok := m.contentCache[me.ContentID]; !ok {
				return m, fetchContentCmd(me.ContentID)
			}
		}
		return m, nil
	case contentLoadedMsg:
		if m.contentCache == nil {
			m.contentCache = map[string]pgdao.ContentRecord{}
		}
		m.contentCache[msg.id] = msg.rec
		return m, nil
	case msgErrMsg:
		m.err = msg.err.Error()
		return m, nil
	}
	return m, nil
}

func (m msgActiveModel) View() string {
	var b strings.Builder
	b.WriteString(mStyleHeader.Render("Messages Viewer") + "\n")
	b.WriteString(mStyleLabel.Render("Conversation: ") + mStyleValue.Render(m.conversation) + "\n")
	b.WriteString(mStyleLabel.Render("Role: ") + mStyleValue.Render(m.role) + "\n")
	// Experiment header
	expID := ""
	expCreated := ""
	if len(m.experiments) > 0 && m.expIdx >= 0 && m.expIdx < len(m.experiments) {
		expID = m.experiments[m.expIdx].ID
		if c := strings.TrimSpace(m.experiments[m.expIdx].Created); c != "" {
			expCreated = c
		}
	}
	newest := ""
	if m.expIdx == 0 {
		newest = " (newest ✨)"
	}
	if expCreated != "" {
		b.WriteString(mStyleLabel.Render("Experiment: "))
		b.WriteString(mStyleValue.Render(fmt.Sprintf("[%d/%d] %s (created %s)", m.expIdx+1, len(m.experiments), expID, expCreated)))
		if newest != "" {
			b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Italic(true).Render(" " + newest))
		}
		b.WriteString("\n")
	} else {
		b.WriteString(mStyleLabel.Render("Experiment: "))
		b.WriteString(mStyleValue.Render(fmt.Sprintf("[%d/%d] %s", m.expIdx+1, len(m.experiments), expID)))
		if newest != "" {
			b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Italic(true).Render(" " + newest))
		}
		b.WriteString("\n")
	}
	// Divider and help
	b.WriteString(mStyleDivider.Render(strings.Repeat("─", 60)) + "\n")
	b.WriteString(mStyleHelp.Render("Keys: ↑/k, ↓/j, n=next exp, r=refresh, q") + "\n")

	if len(m.messages) == 0 {
		b.WriteString("No messages found.\n")
		return b.String()
	}
	// List
	for i, me := range m.messages {
		cursor := "  "
		if i == m.cursor {
			cursor = mStyleCursor.Render("> ")
		}
		created := me.Created.Format(time.RFC3339)
		// Show status and created
		fmt.Fprintf(&b, "%s%s %s %s\n", cursor, colorStatus(me.Status), mStyleValue.Render(created), mStyleValue.Render(me.ID))
	}
	// Error line
	if m.err != "" {
		b.WriteString("\n")
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Render("Error: "))
		b.WriteString(m.err)
		b.WriteString("\n")
	}
	// Details for selected
	if m.cursor >= 0 && m.cursor < len(m.messages) {
		me := m.messages[m.cursor]
		b.WriteString(mStyleDivider.Render(strings.Repeat("─", 60)) + "\n")
		b.WriteString(mStyleHeader.Render("Details") + "\n")
		b.WriteString(mStyleLabel.Render("ID: ") + mStyleValue.Render(me.ID) + "\n")
		b.WriteString(mStyleLabel.Render("Status: ") + mStyleValue.Render(me.Status) + "\n")
		b.WriteString(mStyleLabel.Render("Created: ") + mStyleValue.Render(me.Created.Format(time.RFC3339)) + "\n")
		if me.ExperimentID.Valid {
			b.WriteString(mStyleLabel.Render("Experiment: ") + mStyleValue.Render(me.ExperimentID.String) + "\n")
		}
		if me.FromTaskID.Valid {
			b.WriteString(mStyleLabel.Render("From.task: ") + mStyleValue.Render(me.FromTaskID.String) + "\n")
		}
		cr, ok := m.contentCache[me.ContentID]
		if !ok {
			b.WriteString(mStyleLabel.Render("Text: ") + mStyleValue.Render("(loading…)") + "\n")
		} else if strings.TrimSpace(cr.TextContent) != "" {
			b.WriteString(mStyleLabel.Render("Text: ") + "\n")
			// Render up to first 6 lines of text content
			lines := strings.Split(strings.TrimSpace(cr.TextContent), "\n")
			max := 6
			if len(lines) < max {
				max = len(lines)
			}
			for i := 0; i < max; i++ {
				b.WriteString(mStyleValue.Render(lines[i]) + "\n")
			}
			if len(lines) > max {
				b.WriteString(mStyleValue.Render("…") + "\n")
			}
		}
	}
	return b.String()
}

// Messages for commands and helpers
type msgRefreshMsg struct{ messages []pgdao.MessageEvent }
type contentLoadedMsg struct {
	id  string
	rec pgdao.ContentRecord
}
type msgErrMsg struct{ err error }

func refreshMsgsCmd(role, experimentID string) tea.Cmd {
	return func() tea.Msg {
		cfg, err := cfgpkg.Load()
		if err != nil {
			return msgErrMsg{err}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		db, err := pgdao.OpenApp(ctx, cfg)
		if err != nil {
			return msgErrMsg{err}
		}
		defer db.Close()
		rows, err := pgdao.ListMessages(ctx, db, role, experimentID, "", "", 100, 0)
		if err != nil {
			return msgErrMsg{err}
		}
		return msgRefreshMsg{messages: rows}
	}
}

func fetchContentCmd(contentID string) tea.Cmd {
	return func() tea.Msg {
		cfg, err := cfgpkg.Load()
		if err != nil {
			return msgErrMsg{err}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		db, err := pgdao.OpenApp(ctx, cfg)
		if err != nil {
			return msgErrMsg{err}
		}
		defer db.Close()
		rec, err := pgdao.GetContent(ctx, db, contentID)
		if err != nil {
			return msgErrMsg{err}
		}
		return contentLoadedMsg{id: contentID, rec: rec}
	}
}

func colorStatus(status string) string {
	s := strings.ToLower(strings.TrimSpace(status))
	green := lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	red := lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	yellow := lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	grey := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	switch s {
	case "ok", "sent", "success":
		return green.Render(status)
	case "error", "failed", "ko":
		return red.Render(status)
	case "todo", "pending", "ingested":
		return yellow.Render(status)
	default:
		return grey.Render(status)
	}
}
