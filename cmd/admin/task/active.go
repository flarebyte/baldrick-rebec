package task

import (
    "bytes"
    "context"
    "database/sql"
    "errors"
    "fmt"
    "encoding/json"
    "strings"
    "time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	cfgpkg "github.com/flarebyte/baldrick-rebec/internal/config"
	pgdao "github.com/flarebyte/baldrick-rebec/internal/dao/postgres"
	"github.com/spf13/cobra"
)

var (
	flagTaskActiveConversation string
)

// Styles
var (
	tStyleHeader  = lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Bold(true)
	tStyleLabel   = lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Bold(true)
	tStyleValue   = lipgloss.NewStyle().Foreground(lipgloss.Color("7"))
	tStyleHelp    = lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Italic(true)
	tStyleCursor  = lipgloss.NewStyle().Foreground(lipgloss.Color("13")).Bold(true)
	tStyleDivider = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
)

// activeCmd is an interactive task browser with search, per-role, cycling experiments for context.
var activeCmd = &cobra.Command{
	Use:   "active",
	Short: "Interactive list of tasks for the conversation's role, with search",
	RunE: func(cmd *cobra.Command, args []string) error {
		if strings.TrimSpace(flagTaskActiveConversation) == "" {
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

		// Resolve conversation and role
		conv, err := pgdao.GetConversationByID(ctx, db, flagTaskActiveConversation)
		if err != nil {
			return err
		}
		role := strings.TrimSpace(conv.RoleName)
		if role == "" {
			return errors.New("conversation has no role_name")
		}

		// Experiments for context (cycle like testcase active)
		exps, err := pgdao.ListExperiments(ctx, db, conv.ID, 6, 0)
		if err != nil {
			return err
		}
		// Initial tasks
		ts, err := pgdao.ListTasks(ctx, db, "", role, 100, 0)
		if err != nil {
			return err
		}
		m := newTaskActiveModel(conv.ID, role, exps, 0, ts)
		if _, err := tea.NewProgram(m).Run(); err != nil {
			return err
		}
		return nil
	},
}

func init() {
	TaskCmd.AddCommand(activeCmd)
	activeCmd.Flags().StringVar(&flagTaskActiveConversation, "conversation", "", "Conversation ID (required)")
}

// Model
type taskActiveModel struct {
	conversation string
	role         string
	experiments  []pgdao.Experiment
	expIdx       int
	tasks        []pgdao.Task
	cursor       int
	quitting     bool
	err          string

	// Search
    search      string
    inSearch    bool
    searchInput string

    // Run feedback
    running  bool
    lastRun  string
}

func newTaskActiveModel(conversation, role string, exps []pgdao.Experiment, expIdx int, tasks []pgdao.Task) taskActiveModel {
	if expIdx < 0 || expIdx >= len(exps) {
		expIdx = 0
	}
	return taskActiveModel{conversation: conversation, role: role, experiments: exps, expIdx: expIdx, tasks: tasks}
}

func (m taskActiveModel) Init() tea.Cmd { return nil }

func (m taskActiveModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.inSearch {
			switch msg.Type {
			case tea.KeyEnter:
				m.search = m.searchInput
				m.inSearch = false
				m.cursor = 0
				return m, refreshTasksCmd(m.role, m.search)
			case tea.KeyEsc:
				m.inSearch = false
				m.searchInput = m.search
				return m, nil
			case tea.KeyBackspace, tea.KeyDelete, tea.KeyCtrlH:
				if len(m.searchInput) > 0 {
					m.searchInput = m.searchInput[:len(m.searchInput)-1]
				}
				return m, nil
			case tea.KeySpace:
				m.searchInput += " "
				return m, nil
			case tea.KeyRunes:
				if len(msg.Runes) > 0 {
					m.searchInput += string(msg.Runes)
				}
				return m, nil
			default:
				return m, nil
			}
		}
		switch msg.String() {
		case "ctrl+c", "q":
			m.quitting = true
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
			return m, nil
        case "down", "j":
            if m.cursor < len(m.tasks)-1 {
                m.cursor++
            }
            return m, nil
        case "/":
			m.inSearch = true
			m.searchInput = m.search
			return m, nil
        case "r":
            return m, refreshTasksCmd(m.role, m.search)
        case "enter":
            if m.cursor >= 0 && m.cursor < len(m.tasks) {
                var expID string
                if len(m.experiments) > 0 && m.expIdx >= 0 && m.expIdx < len(m.experiments) { expID = m.experiments[m.expIdx].ID }
                m.running = true; m.lastRun = ""
                return m, runTaskCmd(m.tasks[m.cursor].Variant, expID)
            }
            return m, nil
		case "n":
			if len(m.experiments) > 0 {
				m.expIdx = (m.expIdx + 1) % len(m.experiments)
				m.cursor = 0
				return m, refreshTasksCmd(m.role, m.search)
			}
			return m, nil
		}
	case taskRefreshMsg:
		m.tasks = msg.tasks
		if m.cursor >= len(m.tasks) {
			if len(m.tasks) == 0 {
				m.cursor = 0
			} else {
				m.cursor = len(m.tasks) - 1
			}
		}
		return m, nil
    	case taskErrMsg:
    		m.err = msg.err.Error()
    		return m, nil
    	case taskRunDoneMsg:
    		m.running = false
    		if msg.err != nil {
    			m.lastRun = fmt.Sprintf("failed: %v", msg.err)
    		} else {
    			m.lastRun = fmt.Sprintf("%s (exit=%d, msg=%s)", msg.status, msg.exitCode, msg.messageID)
    		}
    		return m, nil
    	}
    	return m, nil
    }

func (m taskActiveModel) View() string {
	var b strings.Builder
	b.WriteString(tStyleHeader.Render("Tasks Viewer") + "\n")
	b.WriteString(tStyleLabel.Render("Conversation: ") + tStyleValue.Render(m.conversation) + "\n")
	b.WriteString(tStyleLabel.Render("Role: ") + tStyleValue.Render(m.role) + "\n")
	// Experiment header for context
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
		b.WriteString(tStyleLabel.Render("Experiment: "))
		b.WriteString(tStyleValue.Render(fmt.Sprintf("[%d/%d] %s (created %s)", m.expIdx+1, len(m.experiments), expID, expCreated)))
	} else {
		b.WriteString(tStyleLabel.Render("Experiment: "))
		b.WriteString(tStyleValue.Render(fmt.Sprintf("[%d/%d] %s", m.expIdx+1, len(m.experiments), expID)))
	}
	if newest != "" {
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Italic(true).Render(" " + newest))
	}
	b.WriteString("\n")

	// Search line
	if m.inSearch {
		b.WriteString(tStyleLabel.Render("Search*: ") + tStyleValue.Render(m.searchInput) + "\n")
		b.WriteString(tStyleHelp.Render("enter=apply, esc=cancel") + "\n")
	} else {
		b.WriteString(tStyleLabel.Render("Search: ") + tStyleValue.Render(m.search) + "\n")
	}

    b.WriteString(tStyleDivider.Render(strings.Repeat("─", 60)) + "\n")
    b.WriteString(tStyleHelp.Render("Keys: ↑/k, ↓/j, /=search, n=next exp, r=refresh, enter=run, q") + "\n")

	if len(m.tasks) == 0 {
		b.WriteString("No tasks.\n")
		return b.String()
	}
	for i, t := range m.tasks {
		cursor := "  "
		if i == m.cursor {
			cursor = tStyleCursor.Render("> ")
		}
		title := t.Variant
		if t.Title.Valid && strings.TrimSpace(t.Title.String) != "" {
			title += " — " + strings.TrimSpace(t.Title.String)
		}
		fmt.Fprintf(&b, "%s%s\n", cursor, tStyleValue.Render(title))
	}

	// Details for selected task
	if m.cursor >= 0 && m.cursor < len(m.tasks) {
		t := m.tasks[m.cursor]
		b.WriteString(tStyleDivider.Render(strings.Repeat("─", 60)) + "\n")
		b.WriteString(tStyleHeader.Render("Task Details") + "\n")
		b.WriteString(tStyleLabel.Render("ID: ") + tStyleValue.Render(t.ID) + "\n")
		b.WriteString(tStyleLabel.Render("Variant: ") + tStyleValue.Render(t.Variant) + "\n")
		b.WriteString(tStyleLabel.Render("Command: ") + tStyleValue.Render(t.Command) + "\n")
		if t.Title.Valid {
			b.WriteString(tStyleLabel.Render("Title: ") + tStyleValue.Render(t.Title.String) + "\n")
		}
		if t.Description.Valid {
			b.WriteString(tStyleLabel.Render("Description: ") + tStyleValue.Render(t.Description.String) + "\n")
		}
		if t.Motivation.Valid {
			b.WriteString(tStyleLabel.Render("Motivation: ") + tStyleValue.Render(t.Motivation.String) + "\n")
		}
		if t.Notes.Valid {
			b.WriteString(tStyleLabel.Render("Notes: ") + tStyleValue.Render(t.Notes.String) + "\n")
		}
		if t.Shell.Valid {
			b.WriteString(tStyleLabel.Render("Shell: ") + tStyleValue.Render(t.Shell.String) + "\n")
		}
		if t.Timeout.Valid {
			b.WriteString(tStyleLabel.Render("Timeout: ") + tStyleValue.Render(t.Timeout.String) + "\n")
		}
		if t.ToolWorkspaceID.Valid {
			b.WriteString(tStyleLabel.Render("Tool.workspace: ") + tStyleValue.Render(t.ToolWorkspaceID.String) + "\n")
		}
		if t.Level.Valid {
			b.WriteString(tStyleLabel.Render("Level: ") + tStyleValue.Render(t.Level.String) + "\n")
		}
		b.WriteString(tStyleLabel.Render("Archived: ") + tStyleValue.Render(fmt.Sprintf("%v", t.Archived)) + "\n")
        if t.Created.Valid {
            b.WriteString(tStyleLabel.Render("Created: ") + tStyleValue.Render(t.Created.Time.Format(time.RFC3339)) + "\n")
        }
        if m.running {
            b.WriteString(tStyleLabel.Render("Run: ") + tStyleValue.Render("running…") + "\n")
        } else if strings.TrimSpace(m.lastRun) != "" {
            b.WriteString(tStyleLabel.Render("Run: ") + tStyleValue.Render(m.lastRun) + "\n")
        }
    }
    return b.String()
}

// Messages & commands
type taskRefreshMsg struct{ tasks []pgdao.Task }
type taskErrMsg struct{ err error }
type taskRunDoneMsg struct{ status string; exitCode int; messageID string; err error }

func refreshTasksCmd(role, search string) tea.Cmd {
	return func() tea.Msg {
		cfg, err := cfgpkg.Load()
		if err != nil {
			return taskErrMsg{err}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		db, err := pgdao.OpenApp(ctx, cfg)
		if err != nil {
			return taskErrMsg{err}
		}
		defer db.Close()
		var rows []pgdao.Task
		if strings.TrimSpace(search) != "" {
			rows, err = pgdao.ListTasksSearch(ctx, db, role, search, 100, 0)
		} else {
			rows, err = pgdao.ListTasks(ctx, db, "", role, 100, 0)
		}
		if err != nil {
			return taskErrMsg{err}
		}
		return taskRefreshMsg{tasks: rows}
	}
}

// (duplicate Update method removed)

// runTaskCmd executes the current task (script name 'run') under the selected experiment context
func runTaskCmd(variant string, experimentID string) tea.Cmd {
    return func() tea.Msg {
        cfg, err := cfgpkg.Load()
        if err != nil { return taskRunDoneMsg{err: err} }
        ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
        defer cancel()
        db, err := pgdao.OpenApp(ctx, cfg)
        if err != nil { return taskRunDoneMsg{err: err} }
        defer db.Close()
        // Resolve task by variant
        tk, err := pgdao.GetTaskByVariant(ctx, db, variant)
        if err != nil { return taskRunDoneMsg{err: err} }
        // Resolve default script attachment 'run'
        scr, err := pgdao.ResolveTaskScript(ctx, db, tk.ID, "run")
        if err != nil { return taskRunDoneMsg{err: fmt.Errorf("no 'run' script attached: %w", err)} }
        // Timeout
        toDur, err := chooseTimeout("", tk.Timeout.String)
        if err != nil { return taskRunDoneMsg{err: err} }
        if toDur <= 0 { toDur = 5 * time.Minute }
        // Role for message
        roleForMessage := strings.TrimSpace(tk.RoleName)
        if strings.TrimSpace(experimentID) != "" {
            if exp, e1 := pgdao.GetExperimentByID(ctx, db, experimentID); e1 == nil && exp != nil {
                if conv, e2 := pgdao.GetConversationByID(ctx, db, exp.ConversationID); e2 == nil && conv != nil {
                    if rn := strings.TrimSpace(conv.RoleName); rn != "" { roleForMessage = rn }
                }
            }
        }
        // Start message
        startText := fmt.Sprintf("starting task %s (shell=%s, timeout=%s)", tk.Variant, valueOr(tk.Shell.String, "bash"), toDur)
        metaStart := map[string]any{"variant": tk.Variant, "status": "starting", "timeout": toDur.String(), "shell": valueOr(tk.Shell.String, "bash")}
        metaStartJSON, _ := json.Marshal(metaStart)
        startCID, err := pgdao.InsertContent(ctx, db, startText, metaStartJSON)
        if err != nil { return taskRunDoneMsg{err: err} }
        ev := &pgdao.MessageEvent{ContentID: startCID, Status: "starting", Tags: map[string]any{"task": true, "run": true}}
        if roleForMessage != "" { ev.RoleName = roleForMessage }
        if strings.TrimSpace(tk.ID) != "" { ev.FromTaskID = sql.NullString{String: tk.ID, Valid: true} }
        if strings.TrimSpace(experimentID) != "" { ev.ExperimentID = sql.NullString{String: experimentID, Valid: true} }
        msgID, err := pgdao.InsertMessageEvent(ctx, db, ev)
        if err != nil { return taskRunDoneMsg{err: err} }
        // Execute
        body, err := pgdao.GetScriptContent(ctx, db, scr.ScriptContentID)
        if err != nil { return taskRunDoneMsg{err: err} }
        runCtx, cancelRun := context.WithTimeout(context.Background(), toDur)
        defer cancelRun()
        cmdExec, interpreter := buildCommand(runCtx, tk, body)
        var outBuf, errBuf bytes.Buffer
        cmdExec.Stdout = &outBuf; cmdExec.Stderr = &errBuf
        start := time.Now()
        runErr := cmdExec.Run()
        dur := time.Since(start)
        status := "succeeded"; exitCode := 0; errMsg := ""
        if runErr != nil { status = "failed"; exitCode = -1; errMsg = runErr.Error() }
        compMeta := map[string]any{"variant": tk.Variant, "status": status, "duration": dur.String(), "exit_code": exitCode, "shell": interpreter}
        compMetaJSON, _ := json.Marshal(compMeta)
        content := buildCompletionContent(tk, interpreter, dur, exitCode, &outBuf, &errBuf, errMsg)
        compCID, err := pgdao.InsertContent(context.Background(), db, content, compMetaJSON)
        if err != nil { return taskRunDoneMsg{err: err} }
        upd := pgdao.MessageEvent{ContentID: compCID, Status: status}
        if errMsg != "" { upd.ErrorMessage = sql.NullString{String: errMsg, Valid: true} }
        if err := pgdao.UpdateMessageEvent(context.Background(), db, msgID, upd); err != nil { return taskRunDoneMsg{err: err} }
        return taskRunDoneMsg{status: status, exitCode: exitCode, messageID: msgID}
    }
}
