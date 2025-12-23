package testcase

import (
    "context"
    "errors"
    "fmt"
    "os"
    "strings"
    "time"

    cfgpkg "github.com/flarebyte/baldrick-rebec/internal/config"
    pgdao "github.com/flarebyte/baldrick-rebec/internal/dao/postgres"
    "github.com/charmbracelet/lipgloss"
    tea "github.com/charmbracelet/bubbletea"
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

        // Fetch last experiment for the conversation
        exps, err := pgdao.ListExperiments(ctx, db, conv.ID, 1, 0)
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

        // Build lists for the TUI
        titles := make([]string, 0, len(tcs))
        statuses := make([]string, 0, len(tcs))
        for _, t := range tcs {
            titles = append(titles, t.Title)
            statuses = append(statuses, t.Status)
        }

        // Launch Bubble Tea TUI (list of titles)
        fmt.Fprintf(os.Stderr, "testcase active: conversation=%s role=%s experiment=%s count=%d\n", conv.ID, role, exp.ID, len(titles))
        m := newActiveModel(conv.ID, role, exp.ID, titles, statuses)
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
    experiment   string
    testcaseTitles []string
    testcaseStatuses []string
    cursor       int
    quitting     bool
}

func newActiveModel(conversation, role, experiment string, testcaseTitles []string, testcaseStatuses []string) activeModel {
    return activeModel{conversation: conversation, role: role, experiment: experiment, testcaseTitles: testcaseTitles, testcaseStatuses: testcaseStatuses}
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
            if m.cursor < len(m.testcaseTitles)-1 {
                m.cursor++
            }
        case "enter":
            // In future: open details. For now, just quit.
            m.quitting = true
            return m, tea.Quit
        }
    }
    return m, nil
}

func (m activeModel) View() string {
    var b strings.Builder
    fmt.Fprintf(&b, "Conversation: %s\n", m.conversation)
    fmt.Fprintf(&b, "Role: %s\n", m.role)
    fmt.Fprintf(&b, "Experiment: %s\n\n", m.experiment)
    if len(m.testcaseTitles) == 0 {
        b.WriteString("No testcases found. Press q to quit.\n")
        return b.String()
    }
    b.WriteString("Testcases (↑/k, ↓/j, enter, q):\n")
    for i, it := range m.testcaseTitles {
        cursor := "  "
        if i == m.cursor {
            cursor = "> "
        }
        var statusIcon string
        if i < len(m.testcaseStatuses) {
            statusIcon = formatStatus(m.testcaseStatuses[i])
        }
        if statusIcon != "" {
            fmt.Fprintf(&b, "%s%s %s\n", cursor, statusIcon, it)
        } else {
            fmt.Fprintf(&b, "%s%s\n", cursor, it)
        }
    }
    return b.String()
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
