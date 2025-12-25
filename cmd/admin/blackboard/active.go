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

var (
	flagBBActiveSearch string
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
		boards, err := pgdao.ListBlackboardsWithRefs(ctx, db, role, 10, 0, flagBBActiveSearch)
		if err != nil {
			return err
		}

		fmt.Fprintf(os.Stderr, "blackboard active: role=%s boards=%d roles=%d search=%q\n", role, len(boards), len(roleNames), flagBBActiveSearch)
		m := newBBActiveModel(roleNames, role, boards, flagBBActiveSearch)
		if _, err := tea.NewProgram(m).Run(); err != nil {
			return err
		}
		return nil
	},
}

func init() {
	BlackboardCmd.AddCommand(activeCmd)
	activeCmd.Flags().StringVar(&flagBBActiveSearch, "search", "", "Optional search string (matches store/project fields)")
}

// Model
type bbActiveModel struct {
	roles       []string
	roleIdx     int
	boards      []pgdao.BlackboardWithRefs
	cursor      int
	quitting    bool
	err         string
	search      string
	inSearch    bool
	searchInput string
	inBoard     bool
	boardID     string
	boardTitle  string
	stickies    []pgdao.Stickie
	stickCursor int
	// In-board filters
	topicOptions []string // 0:"any", others topic names
	topicIdx     int
	noteSearch   string
	inNoteSearch bool
	noteInput    string
	// Multi-select mode in board view
	inSelect bool
	selected map[string]bool // stickieID -> selected
}

func newBBActiveModel(roles []string, currentRole string, boards []pgdao.BlackboardWithRefs, search string) bbActiveModel {
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
	return bbActiveModel{roles: roles, roleIdx: idx, boards: boards, search: search, inSearch: false, searchInput: search, selected: map[string]bool{}}
}

func (m bbActiveModel) Init() tea.Cmd { return nil }

func (m bbActiveModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.inBoard {
			// In-board note search editing has priority
			if m.inNoteSearch {
				switch msg.Type {
				case tea.KeyEnter:
					m.noteSearch = m.noteInput
					m.inNoteSearch = false
					m.stickCursor = 0
					return m, nil
				case tea.KeyEsc:
					m.inNoteSearch = false
					m.noteInput = m.noteSearch
					return m, nil
				case tea.KeyBackspace, tea.KeyDelete, tea.KeyCtrlH:
					if len(m.noteInput) > 0 {
						m.noteInput = m.noteInput[:len(m.noteInput)-1]
					}
					return m, nil
				case tea.KeySpace:
					// Explicitly accept spaces in search text
					m.noteInput += " "
					return m, nil
				case tea.KeyRunes:
					if len(msg.Runes) > 0 {
						m.noteInput += string(msg.Runes)
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
				if m.stickCursor > 0 {
					m.stickCursor--
				}
				return m, nil
			case "down", "j":
				if m.stickCursor < len(m.filteredStickyIndices())-1 {
					m.stickCursor++
				}
				return m, nil
			case "/":
				m.inNoteSearch = true
				m.noteInput = m.noteSearch
				return m, nil
			case "t":
				if len(m.topicOptions) == 0 {
					m.recomputeTopicOptions()
				}
				if len(m.topicOptions) > 0 {
					m.topicIdx = (m.topicIdx + 1) % len(m.topicOptions)
					if m.stickCursor >= len(m.filteredStickyIndices()) {
						m.stickCursor = 0
					}
				}
				return m, nil
			case "r":
				return m, refreshStickiesCmd(m.boardID)
			case "b", "esc":
				m.inBoard = false
				return m, nil
			case "m":
				// Toggle multi-select mode
				m.inSelect = !m.inSelect
				return m, nil
			case " ":
				// Toggle selection for current stickie when in select mode
				if m.inSelect {
					idxs := m.filteredStickyIndices()
					if m.stickCursor >= 0 && m.stickCursor < len(idxs) {
						s := m.stickies[idxs[m.stickCursor]]
						if m.selected == nil {
							m.selected = map[string]bool{}
						}
						m.selected[s.ID] = !m.selected[s.ID]
					}
					return m, nil
				}
			case "a":
				// Select all visible when in select mode
				if m.inSelect {
					if m.selected == nil {
						m.selected = map[string]bool{}
					}
					for _, i := range m.filteredStickyIndices() {
						m.selected[m.stickies[i].ID] = true
					}
					return m, nil
				}
			case "n":
				// Clear selection
				if m.inSelect {
					m.selected = map[string]bool{}
					return m, nil
				}
			default:
				return m, nil
			}
		}
		if m.inSearch {
			switch msg.Type {
			case tea.KeyEnter:
				m.search = m.searchInput
				m.inSearch = false
				m.cursor = 0
				return m, refreshBoardsCmd(m.roles[m.roleIdx], m.search)
			case tea.KeyEsc:
				m.inSearch = false
				m.searchInput = m.search
				return m, nil
			case tea.KeyBackspace, tea.KeyDelete, tea.KeyCtrlH:
				if len(m.searchInput) > 0 {
					m.searchInput = m.searchInput[:len(m.searchInput)-1]
				}
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
		case "down", "j":
			if m.cursor < len(m.boards)-1 {
				m.cursor++
			}
		case "/":
			m.inSearch = true
			m.searchInput = m.search
			return m, nil
		case "enter":
			if m.cursor >= 0 && m.cursor < len(m.boards) {
				m.inBoard = true
				bb := m.boards[m.cursor]
				m.boardID = bb.ID
				m.boardTitle = firstNonEmpty(strOrNull(bb.ProjectName), strOrNull(bb.ConversationTitle), strOrNull(bb.StoreTitle), bb.StoreID)
				if strings.TrimSpace(m.boardTitle) == "" {
					m.boardTitle = m.boardID
				}
				m.stickCursor = 0
				return m, refreshStickiesCmd(m.boardID)
			}
		case "n":
			// Next role (cycle)
			if len(m.roles) > 0 {
				m.err = ""
				m.roleIdx = (m.roleIdx + 1) % len(m.roles)
				m.cursor = 0
				return m, refreshBoardsCmd(m.roles[m.roleIdx], m.search)
			}
		case "r":
			// Refresh current role
			m.err = ""
			return m, refreshBoardsCmd(m.roles[m.roleIdx], m.search)
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
	case stickRefreshMsg:
		m.stickies = msg.stickies
		m.recomputeTopicOptions()
		if m.stickCursor >= len(m.stickies) {
			m.stickCursor = len(m.stickies) - 1
			if m.stickCursor < 0 {
				m.stickCursor = 0
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
	if m.inBoard {
		b.WriteString(bStyleLabel.Render("Board: ") + bStyleValue.Render(m.boardTitle) + "\n")
		b.WriteString(bStyleHelp.Render("ID: ") + bStyleValue.Render(m.boardID) + "\n")
		// Filter/search section
		topic := "any"
		if m.topicIdx > 0 && m.topicIdx < len(m.topicOptions) {
			topic = m.topicOptions[m.topicIdx]
		}
		b.WriteString(bStyleLabel.Render("Filter.topic: ") + bStyleValue.Render(topic) + bStyleHelp.Render(" (t=cycle)") + "\n")
		if m.inNoteSearch {
			b.WriteString(bStyleLabel.Render("Search.note*: ") + bStyleValue.Render(m.noteInput) + "\n")
			b.WriteString(bStyleHelp.Render("enter=apply, esc=cancel") + "\n")
		} else {
			b.WriteString(bStyleLabel.Render("Search.note: ") + bStyleValue.Render(m.noteSearch) + bStyleHelp.Render(" (/ to edit)") + "\n")
		}
		b.WriteString(bStyleDivider.Render(strings.Repeat("─", 60)) + "\n")
		b.WriteString(bStyleHelp.Render("Keys: ↑/k, ↓/j, /=search, t=topic, m=multi-select, space=toggle, a=all, n=none, b/esc=back, r=refresh, q") + "\n")
		filtered := m.filteredStickyIndices()
		if len(m.stickies) == 0 || len(filtered) == 0 {
			b.WriteString("No stickies.\n")
			return b.String()
		}
		for i, idx := range filtered {
			s := m.stickies[idx]
			cursor := "  "
			if i == m.stickCursor {
				cursor = bStyleCursor.Render("> ")
			}
			title := stickieTitle(s)
			// Selection marker
			if m.inSelect {
				mark := "[ ]"
				if m.selected != nil && m.selected[s.ID] {
					mark = "[x]"
				}
				fmt.Fprintf(&b, "%s%s %s\n", cursor, mark, bStyleValue.Render(title))
			} else {
				fmt.Fprintf(&b, "%s%s\n", cursor, bStyleValue.Render(title))
			}
		}
		// Details for selected stickie
		if m.stickCursor >= 0 && m.stickCursor < len(filtered) {
			st := m.stickies[filtered[m.stickCursor]]
			b.WriteString(bStyleDivider.Render(strings.Repeat("─", 60)) + "\n")
			b.WriteString(bStyleHeader.Render("Stickie Details") + "\n")
			b.WriteString(bStyleLabel.Render("ID: ") + bStyleValue.Render(st.ID) + "\n")
			// Complex name
			if strings.TrimSpace(st.ComplexName.Name) != "" {
				name := st.ComplexName.Name
				if strings.TrimSpace(st.ComplexName.Variant) != "" {
					name += "/" + st.ComplexName.Variant
				}
				b.WriteString(bStyleLabel.Render("Name: ") + bStyleValue.Render(name) + "\n")
			}
			// Topic
			if st.TopicName.Valid {
				b.WriteString(bStyleLabel.Render("Topic: ") + bStyleValue.Render(st.TopicName.String) + "\n")
			}
			if st.TopicRoleName.Valid {
				b.WriteString(bStyleLabel.Render("Topic.role: ") + bStyleValue.Render(st.TopicRoleName.String) + "\n")
			}
			// Note
			if st.Note.Valid && strings.TrimSpace(st.Note.String) != "" {
				b.WriteString(bStyleLabel.Render("Note: ") + bStyleValue.Render(st.Note.String) + "\n")
			}
			// Labels
			if len(st.Labels) > 0 {
				b.WriteString(bStyleLabel.Render("Labels: ") + bStyleValue.Render(strings.Join(st.Labels, ", ")) + "\n")
			}
			// Priority / Score
			if st.PriorityLevel.Valid {
				b.WriteString(bStyleLabel.Render("Priority: ") + bStyleValue.Render(st.PriorityLevel.String) + "\n")
			}
			if st.Score.Valid {
				b.WriteString(bStyleLabel.Render("Score: ") + bStyleValue.Render(fmt.Sprintf("%.3f", st.Score.Float64)) + "\n")
			}
			// Created by task
			if st.CreatedByTaskID.Valid {
				b.WriteString(bStyleLabel.Render("Created.by.task: ") + bStyleValue.Render(st.CreatedByTaskID.String) + "\n")
			}
			// Timestamps / edit count / archived
			if st.Created.Valid {
				b.WriteString(bStyleLabel.Render("Created: ") + bStyleValue.Render(st.Created.Time.Format(time.RFC3339)) + "\n")
			}
			if st.Updated.Valid {
				b.WriteString(bStyleLabel.Render("Updated: ") + bStyleValue.Render(st.Updated.Time.Format(time.RFC3339)) + "\n")
			}
			b.WriteString(bStyleLabel.Render("Edits: ") + bStyleValue.Render(fmt.Sprintf("%d", st.EditCount)) + "\n")
			b.WriteString(bStyleLabel.Render("Archived: ") + bStyleValue.Render(fmt.Sprintf("%v", st.Archived)) + "\n")
		}
		// Selected UUIDs line
		if m.inSelect {
			b.WriteString(bStyleDivider.Render(strings.Repeat("─", 60)) + "\n")
			b.WriteString(bStyleHeader.Render("Selected UUIDs") + "\n")
			b.WriteString(bStyleValue.Render(strings.Join(m.selectedIDsInOrder(filtered), " ")) + "\n")
		}
		return b.String()
	}
	// Search line
	if m.inSearch {
		b.WriteString(bStyleLabel.Render("Search*: "))
		b.WriteString(bStyleValue.Render(m.searchInput) + "\n")
		b.WriteString(bStyleHelp.Render("(editing) enter=apply, esc=cancel") + "\n")
	} else {
		b.WriteString(bStyleLabel.Render("Search: ") + bStyleValue.Render(m.search) + "\n")
	}

	// Divider and help
	b.WriteString(bStyleDivider.Render(strings.Repeat("─", 60)) + "\n")
	b.WriteString(bStyleHelp.Render("Keys: ↑/k, ↓/j, /=search, n=next role, r=refresh, enter, q") + "\n")

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
		if bb.StoreDesc.Valid {
			b.WriteString(bStyleStoreLabel.Render("Store.desc: ") + bStyleValue.Render(bb.StoreDesc.String) + "\n")
		}
		if bb.StoreMotivation.Valid {
			b.WriteString(bStyleStoreLabel.Render("Store.motivation: ") + bStyleValue.Render(bb.StoreMotivation.String) + "\n")
		}
		if bb.StoreSecurity.Valid {
			b.WriteString(bStyleStoreLabel.Render("Store.security: ") + bStyleValue.Render(bb.StoreSecurity.String) + "\n")
		}
		if bb.StorePrivacy.Valid {
			b.WriteString(bStyleStoreLabel.Render("Store.privacy: ") + bStyleValue.Render(bb.StorePrivacy.String) + "\n")
		}
		if bb.StoreNotes.Valid {
			b.WriteString(bStyleStoreLabel.Render("Store.notes: ") + bStyleValue.Render(bb.StoreNotes.String) + "\n")
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
		if bb.ProjectNotes.Valid {
			b.WriteString(bStyleProjLabel.Render("Project.notes: ") + bStyleValue.Render(bb.ProjectNotes.String) + "\n")
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
type stickRefreshMsg struct{ stickies []pgdao.Stickie }
type bbErrMsg struct{ err error }

func refreshBoardsCmd(role string, search string) tea.Cmd {
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
		rows, err := pgdao.ListBlackboardsWithRefs(ctx, db, role, 10, 0, search)
		if err != nil {
			return bbErrMsg{err}
		}
		return bbRefreshMsg{boards: rows}
	}
}

func stickieTitle(s pgdao.Stickie) string {
	name := strings.TrimSpace(s.ComplexName.Name)
	if name != "" {
		v := strings.TrimSpace(s.ComplexName.Variant)
		if v != "" {
			return name + "/" + v
		}
		return name
	}
	if s.Note.Valid && strings.TrimSpace(s.Note.String) != "" {
		return s.Note.String
	}
	return s.ID
}

func refreshStickiesCmd(boardID string) tea.Cmd {
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
		rows, err := pgdao.ListStickies(ctx, db, boardID, "", "", 100, 0)
		if err != nil {
			return bbErrMsg{err}
		}
		return stickRefreshMsg{stickies: rows}
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

// Filtering helpers for in-board view
func (m bbActiveModel) filteredStickyIndices() []int {
	out := make([]int, 0, len(m.stickies))
	// Resolve topic filter
	topic := ""
	if m.topicIdx > 0 && m.topicIdx < len(m.topicOptions) {
		topic = m.topicOptions[m.topicIdx]
	}
	q := strings.ToLower(strings.TrimSpace(m.noteSearch))
	for i, s := range m.stickies {
		// Topic filter
		if topic != "" {
			if !s.TopicName.Valid || strings.TrimSpace(s.TopicName.String) != topic {
				continue
			}
		}
		// Note search
		if q != "" {
			note := ""
			if s.Note.Valid {
				note = s.Note.String
			}
			if !strings.Contains(strings.ToLower(note), q) {
				continue
			}
		}
		out = append(out, i)
	}
	return out
}

func (m *bbActiveModel) recomputeTopicOptions() {
	seen := map[string]struct{}{}
	opts := []string{"any"}
	for _, s := range m.stickies {
		if s.TopicName.Valid {
			t := strings.TrimSpace(s.TopicName.String)
			if t == "" {
				continue
			}
			if _, ok := seen[t]; !ok {
				seen[t] = struct{}{}
				opts = append(opts, t)
			}
		}
	}
	m.topicOptions = opts
	if m.topicIdx >= len(m.topicOptions) {
		m.topicIdx = 0
	}
}

// selectedIDsInOrder returns selected IDs in the order they appear in the filtered list
func (m bbActiveModel) selectedIDsInOrder(filtered []int) []string {
	if m.selected == nil || len(m.selected) == 0 {
		return nil
	}
	out := make([]string, 0, len(m.selected))
	for _, i := range filtered {
		id := m.stickies[i].ID
		if m.selected[id] {
			out = append(out, id)
		}
	}
	return out
}
