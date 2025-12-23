package testcase

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	cfgpkg "github.com/flarebyte/baldrick-rebec/internal/config"
	pgdao "github.com/flarebyte/baldrick-rebec/internal/dao/postgres"
	"github.com/spf13/cobra"
)

var (
	flagTCActiveConversation string
)

// activeCmd is an interactive variant of list (dummy implementation).
// For now, it validates required flags and returns a mock payload.
var activeCmd = &cobra.Command{
	Use:   "active",
	Short: "Interactive list of active testcases (mock)",
	RunE: func(cmd *cobra.Command, args []string) error {
		if strings.TrimSpace(flagTCActiveConversation) == "" {
			return errors.New("--conversation is required")
		}

		// Load config and resolve role_name from the conversation
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

		// Resolve conversation and role via DAO
		conv, err := pgdao.GetConversationByID(ctx, db, flagTCActiveConversation)
		if err != nil {
			return err
		}
		role := strings.TrimSpace(conv.RoleName)
		if role == "" {
			return errors.New("conversation has no role_name")
		}

		// Fetch recent experiments for the conversation (up to 6)
		exps, err := pgdao.ListExperiments(ctx, db, conv.ID, 6, 0)
		if err != nil {
			return err
		}
		if len(exps) == 0 {
			return fmt.Errorf("no experiment found for conversation %s", conv.ID)
		}
		exp := exps[0]

		// Fetch testcases for that experiment and role
		tcs, err := pgdao.ListTestcases(ctx, db, role, exp.ID, "", 100, 0)
		if err != nil {
			return err
		}

		// Build model with full testcase records

		// Launch Bubble Tea TUI (list of titles)
		fmt.Fprintf(os.Stderr, "testcase active: conversation=%s role=%s experiments=%d current=%s count=%d\n", conv.ID, role, len(exps), exp.ID, len(tcs))
		m := newActiveModel(conv.ID, role, exps, 0, tcs)
		if _, err := tea.NewProgram(m).Run(); err != nil {
			return err
		}
		return nil
	},
}

// Bubble Tea model for interactive testcase list
type activeModel struct {
	conversation string
	role         string
	experiments  []pgdao.Experiment
	expIdx       int
	testcases    []pgdao.Testcase
	cursor       int
	quitting     bool
	err          string
	errorsOnly   bool
}

func newActiveModel(conversation, role string, experiments []pgdao.Experiment, expIdx int, testcases []pgdao.Testcase) activeModel {
	if expIdx < 0 || expIdx >= len(experiments) {
		expIdx = 0
	}
	return activeModel{conversation: conversation, role: role, experiments: experiments, expIdx: expIdx, testcases: testcases}
}

func (m activeModel) Init() tea.Cmd { return nil }

func (m activeModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
			// move within filtered list
			if m.cursor < len(m.filteredIndices())-1 {
				m.cursor++
			}
		case "enter":
			// Reserved for future: open a detail pane or actions. No-op for now.
		case "r":
			// Refresh testcases for current experiment
			m.err = ""
			return m, refreshForExpCmd(m.role, m.experiments[m.expIdx].ID)
		case "h", "left":
			// Previous experiment (cycle)
			if len(m.experiments) > 0 {
				m.err = ""
				m.expIdx = (m.expIdx - 1 + len(m.experiments)) % len(m.experiments)
				m.cursor = 0
				return m, refreshForExpCmd(m.role, m.experiments[m.expIdx].ID)
			}
		case "l", "right":
			// Next experiment (cycle)
			if len(m.experiments) > 0 {
				m.err = ""
				m.expIdx = (m.expIdx + 1) % len(m.experiments)
				m.cursor = 0
				return m, refreshForExpCmd(m.role, m.experiments[m.expIdx].ID)
			}
		case "e":
			// Toggle errors-only view
			m.errorsOnly = !m.errorsOnly
			// Normalize cursor against new filtered length
			if m.cursor >= len(m.filteredIndices()) {
				m.cursor = 0
			}
			return m, nil
		}
	case refreshMsg:
		m.testcases = msg.testcases
		// find index of experiment in list, if provided
		for i, e := range m.experiments {
			if e.ID == msg.experiment {
				m.expIdx = i
				break
			}
		}
		// Clamp cursor against filtered length after refresh
		if m.cursor >= len(m.filteredIndices()) {
			m.cursor = len(m.filteredIndices()) - 1
			if m.cursor < 0 {
				m.cursor = 0
			}
		}
		return m, nil
	case errMsg:
		m.err = msg.err.Error()
		return m, nil
	}
	return m, nil
}

func (m activeModel) View() string {
	var b strings.Builder
	fmt.Fprintf(&b, "Conversation: %s\n", m.conversation)
	fmt.Fprintf(&b, "Role: %s\n", m.role)
	// Experiment header
	expID := ""
	expCreated := ""
	if len(m.experiments) > 0 && m.expIdx >= 0 && m.expIdx < len(m.experiments) {
		expID = m.experiments[m.expIdx].ID
		if c := strings.TrimSpace(m.experiments[m.expIdx].Created); c != "" {
			expCreated = c
		}
	}
	if expCreated != "" {
		fmt.Fprintf(&b, "Experiment [%d/%d]: %s (created %s)\n", m.expIdx+1, len(m.experiments), expID, expCreated)
	} else {
		fmt.Fprintf(&b, "Experiment [%d/%d]: %s\n", m.expIdx+1, len(m.experiments), expID)
	}
	// Filter status line
	filterLbl := "all"
	if m.errorsOnly {
		filterLbl = "errors-only"
	}
	fmt.Fprintf(&b, "Filter: %s\n\n", filterLbl)

	filtered := m.filteredIndices()
	if len(m.testcases) == 0 || len(filtered) == 0 {
		if m.errorsOnly {
			b.WriteString("No error testcases. Press 'e' to show all; 'q' to quit.\n")
		} else {
			b.WriteString("No testcases found. Press q to quit.\n")
		}
		return b.String()
	}
	b.WriteString("Testcases (↑/k, ↓/j, enter, h/← prev exp, l/→ next exp, r=refresh, e=errors-only, q):\n")
	for i, idx := range filtered {
		tc := m.testcases[idx]
		cursor := "  "
		if i == m.cursor {
			cursor = "> "
		}
		statusIcon := formatStatus(tc.Status)
		indent := indentForLevel(levelFromTestcase(tc))
		fmt.Fprintf(&b, "%s%s%s %s\n", cursor, indent, statusIcon, tc.Title)
	}
	if m.err != "" {
		b.WriteString("\n")
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Render("Error: "))
		b.WriteString(m.err)
		b.WriteString("\n")
	}
	// Details section for selected testcase
	if m.cursor >= 0 && m.cursor < len(filtered) {
		tc := m.testcases[filtered[m.cursor]]
		b.WriteString("\nDetails:\n")
		fmt.Fprintf(&b, "ID: %s\n", tc.ID)
		fmt.Fprintf(&b, "Title: %s\n", tc.Title)
		fmt.Fprintf(&b, "Status: %s\n", tc.Status)
		if tc.Name.Valid {
			fmt.Fprintf(&b, "Name: %s\n", tc.Name.String)
		}
		if tc.Package.Valid {
			fmt.Fprintf(&b, "Package: %s\n", tc.Package.String)
		}
		if tc.Classname.Valid {
			fmt.Fprintf(&b, "Classname: %s\n", tc.Classname.String)
		}
		if tc.File.Valid {
			if tc.Line.Valid {
				fmt.Fprintf(&b, "File: %s:%d\n", tc.File.String, tc.Line.Int64)
			} else {
				fmt.Fprintf(&b, "File: %s\n", tc.File.String)
			}
		}
		if tc.ExecutionTime.Valid {
			fmt.Fprintf(&b, "Execution: %.3fs\n", tc.ExecutionTime.Float64)
		}
		if tc.ErrorMessage.Valid {
			fmt.Fprintf(&b, "Error: %s\n", tc.ErrorMessage.String)
		}
		if tc.Created.Valid {
			fmt.Fprintf(&b, "Created: %s\n", tc.Created.Time.Format(time.RFC3339))
		}
	}
	return b.String()
}

// Helpers
func (m activeModel) filteredIndices() []int {
	if !m.errorsOnly {
		out := make([]int, len(m.testcases))
		for i := range m.testcases {
			out[i] = i
		}
		return out
	}
	out := make([]int, 0, len(m.testcases))
	for i, tc := range m.testcases {
		if isErrorStatus(tc.Status) {
			out = append(out, i)
		}
	}
	return out
}

func isErrorStatus(status string) bool {
	return strings.ToLower(strings.TrimSpace(status)) == "ko"
}

// Messages
type refreshMsg struct {
	testcases  []pgdao.Testcase
	experiment string
}

type errMsg struct{ err error }

// Commands
func refreshForExpCmd(role, experimentID string) tea.Cmd {
	return func() tea.Msg {
		cfg, err := cfgpkg.Load()
		if err != nil {
			return errMsg{err}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		db, err := pgdao.OpenApp(ctx, cfg)
		if err != nil {
			return errMsg{err}
		}
		defer db.Close()
		// Fetch testcases for the provided experiment
		tcs, err := pgdao.ListTestcases(ctx, db, role, experimentID, "", 100, 0)
		if err != nil {
			return errMsg{err}
		}
		return refreshMsg{testcases: tcs, experiment: experimentID}
	}
}

// formatStatus renders a colored status icon or text.
func formatStatus(status string) string {
	s := strings.ToLower(strings.TrimSpace(status))
	green := lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	red := lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	yellow := lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	grey := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	switch s {
	case "ok":
		return green.Render("✔")
	case "ko":
		return red.Render("✗")
	case "todo":
		return yellow.Render("⏳")
	default:
		return grey.Render(status)
	}
}

func init() {
	TestcaseCmd.AddCommand(activeCmd)
	activeCmd.Flags().StringVar(&flagTCActiveConversation, "conversation", "", "Conversation ID (required)")
}

// levelFromTestcase returns the raw level string (e.g., "h1") or empty if unset.
func levelFromTestcase(tc pgdao.Testcase) string {
	if tc.Level.Valid {
		return tc.Level.String
	}
	return ""
}

// indentForLevel computes indentation spaces based on level.
// - Default when empty/invalid: h1 (0 spaces)
// - hN indents (N-1)*2 spaces, capped at h6
func indentForLevel(level string) string {
	s := strings.ToLower(strings.TrimSpace(level))
	n := 1
	if s != "" {
		if strings.HasPrefix(s, "h") {
			s = s[1:]
		}
		if v, err := strconv.Atoi(s); err == nil && v >= 1 {
			n = v
		}
	}
	if n > 6 {
		n = 6
	}
	if n < 1 {
		n = 1
	}
	return strings.Repeat(" ", (n-1)*2)
}
