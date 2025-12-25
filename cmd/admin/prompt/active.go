package prompt

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	cfgpkg "github.com/flarebyte/baldrick-rebec/internal/config"
	pgdao "github.com/flarebyte/baldrick-rebec/internal/dao/postgres"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

// Styles
var (
	pStyleHeader    = lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Bold(true)   // cyan
	pStyleLabel     = lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Bold(true)    // gray
	pStyleValue     = lipgloss.NewStyle().Foreground(lipgloss.Color("7"))               // white-ish
	pStyleHelp      = lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Italic(true)  // dim italic
	pStyleCursor    = lipgloss.NewStyle().Foreground(lipgloss.Color("13")).Bold(true)   // magenta
	pStyleDivider   = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))               // gray line
	pStyleWarn      = lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Bold(true)    // red
	pStyleTCTitle   = lipgloss.NewStyle().Foreground(lipgloss.Color("14")).Italic(true) // cyan italic
	pStyleStickNote = lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Italic(true) // green italic
)

// BlockKind enumerates allowed kinds for blocks
type BlockKind string

const (
	KindText     BlockKind = "text"
	KindTestcase BlockKind = "testcase"
	KindStickie  BlockKind = "stickie"
)

var allKinds = []BlockKind{KindText, KindTestcase, KindStickie}

// DesignBlock represents a prompt designer block
type DesignBlock struct {
	ID       string `json:"id"`
	Kind     string `json:"kind"`
	Value    string `json:"value"`
	Disabled bool   `json:"disabled"`
}

// Command
var activeCmd = &cobra.Command{
	Use:   "active",
	Short: "Interactive prompt designer (admin)",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Seed with a single default section
		m := newPromptModel([]DesignBlock{newDefaultBlock()})
		final, err := tea.NewProgram(m).Run()
		if err != nil {
			return err
		}
		// Export on request
		if got, ok := final.(promptModel); ok && got.export {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(got.blocks)
		}
		return nil
	},
}

func init() {
	PromptCmd.AddCommand(activeCmd)
}

// Model for the designer
type promptModel struct {
	blocks      []DesignBlock
	cursor      int // which block
	quitting    bool
	export      bool // if true, print JSON on exit
	inPreview   bool
	inQuickAdd  bool
	quickBuffer string

	// detail pane selection 0..2 (value, id, disabled)
	detailIdx int
	// inline text editing state
	editing    bool
	editBuffer string
	// which field is being edited (same as detailIdx when editing text)

	// Loaded testcase cache by ID
	tcCache map[string]pgdao.Testcase
	// pending fetch id (set after edit commit)
	pendingTCID string

	// Loaded stickie cache by ID
	stickCache map[string]pgdao.Stickie
	// pending stickie id
	pendingStickID string
}

func newPromptModel(initial []DesignBlock) promptModel {
	if len(initial) == 0 {
		initial = []DesignBlock{newDefaultBlock()}
	}
	return promptModel{blocks: initial, cursor: 0, detailIdx: 0, tcCache: map[string]pgdao.Testcase{}, stickCache: map[string]pgdao.Stickie{}}
}

func newDefaultBlock() DesignBlock {
	return DesignBlock{
		ID:       uuid.NewString(),
		Kind:     string(KindText),
		Value:    "",
		Disabled: false,
	}
}

func (m promptModel) Init() tea.Cmd { return nil }

func (m promptModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Editing mode: capture text
		if m.editing {
			switch msg.Type {
			case tea.KeyEnter:
				// Commit edit; may trigger async fetch
				m = m.commitEdit()
				var cmds []tea.Cmd
				if id := strings.TrimSpace(m.pendingTCID); id != "" {
					cmds = append(cmds, fetchTestcaseCmd(id))
					m.pendingTCID = ""
				}
				if id := strings.TrimSpace(m.pendingStickID); id != "" {
					cmds = append(cmds, fetchStickieCmd(id))
					m.pendingStickID = ""
				}
				if len(cmds) > 0 {
					return m, tea.Batch(cmds...)
				}
				return m, nil
			case tea.KeyEsc:
				// Cancel edit
				m.editing = false
				m.editBuffer = ""
				return m, nil
			case tea.KeyBackspace, tea.KeyDelete, tea.KeyCtrlH:
				if len(m.editBuffer) > 0 {
					m.editBuffer = m.editBuffer[:len(m.editBuffer)-1]
				}
				return m, nil
			case tea.KeyCtrlU:
				m.editBuffer = ""
				return m, nil
			case tea.KeySpace:
				// Accept spaces explicitly (some terminals send KeySpace, not KeyRunes)
				m.editBuffer += " "
				return m, nil
			case tea.KeyRunes:
				if len(msg.Runes) > 0 {
					m.editBuffer += string(msg.Runes)
				}
				return m, nil
			default:
				return m, nil
			}
		}
		// Quick add mode: capture UUID list input
		if m.inQuickAdd {
			switch msg.Type {
			case tea.KeyEnter:
				ids := parseUUIDs(strings.TrimSpace(m.quickBuffer))
				m.inQuickAdd = false
				m.quickBuffer = ""
				if len(ids) == 0 {
					return m, nil
				}
				cmds := make([]tea.Cmd, 0, len(ids))
				for _, id := range ids {
					cmds = append(cmds, detectAndLoadByIDCmd(id))
				}
				return m, tea.Batch(cmds...)
			case tea.KeyEsc:
				m.inQuickAdd = false
				m.quickBuffer = ""
				return m, nil
			case tea.KeyBackspace, tea.KeyDelete, tea.KeyCtrlH:
				if len(m.quickBuffer) > 0 {
					m.quickBuffer = m.quickBuffer[:len(m.quickBuffer)-1]
				}
				return m, nil
			case tea.KeyCtrlU:
				m.quickBuffer = ""
				return m, nil
			case tea.KeySpace:
				m.quickBuffer += " "
				return m, nil
			case tea.KeyRunes:
				if len(msg.Runes) > 0 {
					m.quickBuffer += string(msg.Runes)
				}
				return m, nil
			default:
				return m, nil
			}
		}
		// Normal mode
		switch msg.String() {
		case "ctrl+c", "q":
			m.quitting = true
			return m, tea.Quit
		case "s":
			// Export JSON then quit
			m.export = true
			return m, tea.Quit
		case "p":
			// Toggle Markdown preview
			m.inPreview = !m.inPreview
			return m, nil
		case "u":
			// Enter quick add UUIDs mode
			m.inQuickAdd = true
			m.quickBuffer = ""
			return m, nil
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
			return m, nil
		case "down", "j":
			if m.cursor < len(m.blocks)-1 {
				m.cursor++
			}
			return m, nil
			// Removed tab/shift+tab field cycling to simplify UX
		case "1":
			// Add new text block
			nb := DesignBlock{ID: uuid.NewString(), Kind: string(KindText), Value: "", Disabled: false}
			m.blocks, m.cursor = appendAfter(m.blocks, m.cursor, nb)
			m.detailIdx = 0 // focus value
			return m, nil
		case "d":
			// Delete current
			if len(m.blocks) > 0 {
				idx := m.cursor
				m.blocks = append(m.blocks[:idx], m.blocks[idx+1:]...)
				if m.cursor >= len(m.blocks) {
					m.cursor = len(m.blocks) - 1
				}
				if m.cursor < 0 {
					// empty list: regenerate a default block for convenience
					m.blocks = []DesignBlock{newDefaultBlock()}
					m.cursor = 0
				}
			}
			return m, nil
		case "[":
			// Move current block up
			if m.cursor > 0 && m.cursor < len(m.blocks) {
				i := m.cursor
				m.blocks[i-1], m.blocks[i] = m.blocks[i], m.blocks[i-1]
				m.cursor--
			}
			return m, nil
		case "]":
			// Move current block down
			if m.cursor >= 0 && m.cursor < len(m.blocks)-1 {
				i := m.cursor
				m.blocks[i], m.blocks[i+1] = m.blocks[i+1], m.blocks[i]
				m.cursor++
			}
			return m, nil
		case "enter", "e":
			// Edit value directly
			if len(m.blocks) == 0 {
				return m, nil
			}
			m.detailIdx = 0
			m.editing = true
			m.editBuffer = m.blocks[m.cursor].Value
			return m, nil
		case "i":
			// Edit ID directly
			if len(m.blocks) == 0 {
				return m, nil
			}
			m.detailIdx = 1
			m.editing = true
			m.editBuffer = m.blocks[m.cursor].ID
			return m, nil
		case "x":
			// Toggle disable
			if len(m.blocks) > 0 {
				m.blocks[m.cursor].Disabled = !m.blocks[m.cursor].Disabled
			}
			return m, nil
		}
	case tcLoadedMsg:
		if m.tcCache == nil {
			m.tcCache = map[string]pgdao.Testcase{}
		}
		m.tcCache[msg.id] = msg.tc
		return m, nil
	case stickLoadedMsg:
		if m.stickCache == nil {
			m.stickCache = map[string]pgdao.Stickie{}
		}
		m.stickCache[msg.id] = msg.s
		return m, nil
	case quickAddResult:
		// Append a new block based on detected kind, cache loaded entity
		switch msg.kind {
		case KindTestcase:
			if msg.tc != nil {
				if m.tcCache == nil {
					m.tcCache = map[string]pgdao.Testcase{}
				}
				m.tcCache[msg.id] = *msg.tc
			}
			if !m.hasBlock(KindTestcase, msg.id) {
				nb := DesignBlock{ID: uuid.NewString(), Kind: string(KindTestcase), Value: msg.id, Disabled: false}
				m.blocks = append(m.blocks, nb)
			}
			return m, nil
		case KindStickie:
			if msg.st != nil {
				if m.stickCache == nil {
					m.stickCache = map[string]pgdao.Stickie{}
				}
				m.stickCache[msg.id] = *msg.st
			}
			if !m.hasBlock(KindStickie, msg.id) {
				nb := DesignBlock{ID: uuid.NewString(), Kind: string(KindStickie), Value: msg.id, Disabled: false}
				m.blocks = append(m.blocks, nb)
			}
			return m, nil
		default:
			return m, nil
		}
	case tcErrMsg:
		// silently ignore; user can correct ID
		return m, nil
	}
	return m, nil
}

func (m promptModel) View() string {
	var b strings.Builder
	if m.inPreview {
		b.WriteString(pStyleHeader.Render("Preview (Markdown)") + "\n")
		b.WriteString(pStyleHelp.Render("Keys: p=back, q=quit") + "\n")
		b.WriteString(pStyleDivider.Render(strings.Repeat("─", 60)) + "\n")
		b.WriteString(m.renderPreview())
		return b.String()
	}
	b.WriteString(pStyleHeader.Render("Prompt Designer") + "\n")
	b.WriteString(pStyleHelp.Render("Keys: ↑/k, ↓/j, 1=text, d=del, [=up, ]=down, enter/e=edit value, i=edit id, x=disable, u=quick add UUIDs, p=preview, s=save JSON, q") + "\n")
	b.WriteString(pStyleDivider.Render(strings.Repeat("─", 60)) + "\n")
	if m.inQuickAdd {
		b.WriteString(pStyleLabel.Render("Quick add UUIDs*: "))
		b.WriteString(pStyleValue.Render(m.quickBuffer) + "\n")
		b.WriteString(pStyleHelp.Render("(paste UUIDs separated by spaces) enter=apply, esc=cancel") + "\n")
		b.WriteString(pStyleDivider.Render(strings.Repeat("─", 60)) + "\n")
	}

	// List of blocks
	if len(m.blocks) == 0 {
		b.WriteString("No blocks. Use 1 to add text, or 'u' to quick add UUIDs.\n")
		return b.String()
	}
	for i, blk := range m.blocks {
		cursor := "  "
		if i == m.cursor {
			cursor = pStyleCursor.Render("> ")
		}
		title := m.summarizeBlock(blk)
		if blk.Disabled {
			title += " " + pStyleWarn.Render("(disabled)")
		}
		fmt.Fprintf(&b, "%s%s\n", cursor, pStyleValue.Render(title))
	}

	// Details for selected block
	if m.cursor >= 0 && m.cursor < len(m.blocks) {
		blk := m.blocks[m.cursor]
		b.WriteString(pStyleDivider.Render(strings.Repeat("─", 60)) + "\n")
		b.WriteString(pStyleHeader.Render("Details") + "\n")

		// Field helper for selection highlight
		renderField := func(idx int, label, val string) string {
			if m.detailIdx == idx {
				return pStyleCursor.Render(label) + pStyleValue.Render(val)
			}
			return pStyleLabel.Render(label) + pStyleValue.Render(val)
		}

		// Value first; for testcase/stickie, show resolved title/note if available
		if strings.ToLower(strings.TrimSpace(blk.Kind)) == string(KindTestcase) {
			if tc, ok := m.tcCache[strings.TrimSpace(blk.Value)]; ok && strings.TrimSpace(tc.Title) != "" {
				b.WriteString(renderField(0, "Value: ", blk.Value) + "\n")
				b.WriteString(pStyleLabel.Render("  ↳ Testcase.title: ") + pStyleTCTitle.Render(tc.Title) + "\n")
			} else {
				b.WriteString(renderField(0, "Value: ", blk.Value) + "\n")
			}
		} else if strings.ToLower(strings.TrimSpace(blk.Kind)) == string(KindStickie) {
			if st, ok := m.stickCache[strings.TrimSpace(blk.Value)]; ok && st.Note.Valid && strings.TrimSpace(st.Note.String) != "" {
				b.WriteString(renderField(0, "Value: ", blk.Value) + "\n")
				// Truncate long notes for display
				note := strings.TrimSpace(st.Note.String)
				if len(note) > 80 {
					note = note[:80] + "…"
				}
				b.WriteString(pStyleLabel.Render("  ↳ Stickie.note: ") + pStyleStickNote.Render(note) + "\n")
			} else {
				b.WriteString(renderField(0, "Value: ", blk.Value) + "\n")
			}
		} else {
			b.WriteString(renderField(0, "Value: ", blk.Value) + "\n")
		}
		// Static kind (not editable)
		b.WriteString(pStyleLabel.Render("Kind: ") + pStyleValue.Render(blk.Kind) + "\n")
		// ID and Disabled follow
		b.WriteString(renderField(1, "ID: ", blk.ID) + "\n")
		b.WriteString(renderField(2, "Disabled: ", fmt.Sprintf("%v", blk.Disabled)) + "\n")

		if m.editing {
			b.WriteString(pStyleHelp.Render("Editing: ") + pStyleValue.Render(m.editBuffer) + "\n")
			b.WriteString(pStyleHelp.Render("enter=apply, esc=cancel, ctrl+u=clear"))
			b.WriteString("\n")
		}
	}
	return b.String()
}

// renderPreview concatenates enabled blocks into simple Markdown.
func (m promptModel) renderPreview() string {
	var out strings.Builder
	for _, blk := range m.blocks {
		if blk.Disabled {
			continue
		}
		kind := strings.ToLower(strings.TrimSpace(blk.Kind))
		val := strings.TrimSpace(blk.Value)
		switch kind {
		case string(KindText):
			if val != "" {
				out.WriteString(val)
				out.WriteString("\n\n")
			}
		case string(KindTestcase):
			if val == "" {
				break
			}
			if tc, ok := m.tcCache[val]; ok {
				title := strings.TrimSpace(tc.Title)
				if title != "" {
					out.WriteString(title)
					out.WriteString("\n")
					// Always show status
					if strings.TrimSpace(tc.Status) != "" {
						out.WriteString("- Status: ")
						out.WriteString(strings.TrimSpace(tc.Status))
						out.WriteString("\n")
					}
					// Optional fields
					if tc.Name.Valid && strings.TrimSpace(tc.Name.String) != "" {
						out.WriteString("- Name: ")
						out.WriteString(strings.TrimSpace(tc.Name.String))
						out.WriteString("\n")
					}
					if tc.Package.Valid && strings.TrimSpace(tc.Package.String) != "" {
						out.WriteString("- Package: ")
						out.WriteString(strings.TrimSpace(tc.Package.String))
						out.WriteString("\n")
					}
					if tc.Classname.Valid && strings.TrimSpace(tc.Classname.String) != "" {
						out.WriteString("- Classname: ")
						out.WriteString(strings.TrimSpace(tc.Classname.String))
						out.WriteString("\n")
					}
					if tc.File.Valid && strings.TrimSpace(tc.File.String) != "" {
						out.WriteString("- File: ")
						out.WriteString(strings.TrimSpace(tc.File.String))
						if tc.Line.Valid {
							out.WriteString(":" + fmt.Sprintf("%d", tc.Line.Int64))
						}
						out.WriteString("\n")
					}
					if tc.ErrorMessage.Valid && strings.TrimSpace(tc.ErrorMessage.String) != "" {
						out.WriteString("- Error: ")
						out.WriteString(strings.TrimSpace(tc.ErrorMessage.String))
						out.WriteString("\n")
					}
					out.WriteString("\n")
				}
			}
		case string(KindStickie):
			if val == "" {
				break
			}
			if st, ok := m.stickCache[val]; ok {
				if st.Note.Valid {
					note := strings.TrimSpace(st.Note.String)
					if note != "" {
						out.WriteString(note)
						out.WriteString("\n")
					}
				}
				if st.PriorityLevel.Valid && strings.TrimSpace(st.PriorityLevel.String) != "" {
					out.WriteString("- Priority: ")
					out.WriteString(strings.TrimSpace(st.PriorityLevel.String))
					out.WriteString("\n")
				}
				out.WriteString("\n")
			}
		default:
			if val != "" {
				out.WriteString(val)
				out.WriteString("\n\n")
			}
		}
	}
	return out.String()
}

// hasBlock returns true if a block with the given kind and value already exists
func (m promptModel) hasBlock(kind BlockKind, value string) bool {
	kv := strings.ToLower(strings.TrimSpace(string(kind)))
	vv := strings.TrimSpace(value)
	for _, b := range m.blocks {
		if strings.ToLower(strings.TrimSpace(b.Kind)) == kv && strings.TrimSpace(b.Value) == vv {
			return true
		}
	}
	return false
}

// appendAfter inserts nb after the current cursor; returns new slice and new cursor pointing to the inserted element
func appendAfter(list []DesignBlock, cursor int, nb DesignBlock) ([]DesignBlock, int) {
	if len(list) == 0 || cursor >= len(list)-1 {
		list = append(list, nb)
		return list, len(list) - 1
	}
	idx := cursor + 1
	list = append(list[:idx], append([]DesignBlock{nb}, list[idx:]...)...)
	return list, idx
}

// Quick add: parse space-separated UUIDs
func parseUUIDs(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Fields(s)
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if _, err := uuid.Parse(p); err == nil {
			out = append(out, p)
		}
	}
	return out
}

func (m promptModel) summarizeBlock(b DesignBlock) string {
	switch strings.ToLower(strings.TrimSpace(b.Kind)) {
	case string(KindText):
		v := strings.TrimSpace(b.Value)
		if v == "" {
			return "text: <markdown>"
		}
		if len(v) > 40 {
			v = v[:40] + "…"
		}
		return "text: " + v
	case string(KindTestcase):
		id := strings.TrimSpace(b.Value)
		if id == "" {
			return "testcase: <uuid>"
		}
		// if we have it, append styled title
		if tc, ok := m.tcCache[id]; ok && strings.TrimSpace(tc.Title) != "" {
			return "testcase: " + id + " — " + pStyleTCTitle.Render(tc.Title)
		}
		return "testcase: " + id
	case string(KindStickie):
		if strings.TrimSpace(b.Value) == "" {
			return "stickie: <uuid>"
		}
		id := strings.TrimSpace(b.Value)
		if st, ok := m.stickCache[id]; ok && st.Note.Valid && strings.TrimSpace(st.Note.String) != "" {
			note := strings.TrimSpace(st.Note.String)
			if len(note) > 40 {
				note = note[:40] + "…"
			}
			return "stickie: " + id + " — " + pStyleStickNote.Render(note)
		}
		return "stickie: " + id
	default:
		if strings.TrimSpace(b.Value) != "" {
			return b.Kind + ": " + b.Value
		}
		return b.Kind
	}
}

func nextKind(k BlockKind) BlockKind {
	for i, v := range allKinds {
		if v == k {
			return allKinds[(i+1)%len(allKinds)]
		}
	}
	return allKinds[0]
}

// commitEdit writes editBuffer into the selected field and exits edit mode
func (m promptModel) commitEdit() promptModel {
	if len(m.blocks) == 0 {
		m.editing = false
		m.editBuffer = ""
		return m
	}
	switch m.detailIdx {
	case 0: // value
		m.blocks[m.cursor].Value = m.editBuffer
		// If this is a testcase or stickie block and looks like a UUID, schedule fetch
		blk := m.blocks[m.cursor]
		id := strings.TrimSpace(m.blocks[m.cursor].Value)
		if _, err := uuid.Parse(id); err == nil {
			switch strings.ToLower(strings.TrimSpace(blk.Kind)) {
			case string(KindTestcase):
				m.pendingTCID = id
			case string(KindStickie):
				m.pendingStickID = id
			}
		}
	case 1: // id
		m.blocks[m.cursor].ID = strings.TrimSpace(m.editBuffer)
	}
	m.editing = false
	m.editBuffer = ""
	return m
}

// Messages
type tcLoadedMsg struct {
	id string
	tc pgdao.Testcase
}
type tcErrMsg struct{ err error }
type stickLoadedMsg struct {
	id string
	s  pgdao.Stickie
}
type quickAddResult struct {
	id   string
	kind BlockKind
	tc   *pgdao.Testcase
	st   *pgdao.Stickie
}

// fetchTestcaseCmd loads a testcase by ID from Postgres
func fetchTestcaseCmd(id string) tea.Cmd {
	return func() tea.Msg {
		cfg, err := cfgpkg.Load()
		if err != nil {
			return tcErrMsg{err}
		}
		ctx := contextForShort()
		defer ctx.cancel()
		db, err := pgdao.OpenApp(ctx.ctx, cfg)
		if err != nil {
			return tcErrMsg{err}
		}
		defer db.Close()
		row, err := pgdao.GetTestcaseByID(ctx.ctx, db, id)
		if err != nil {
			return tcErrMsg{err}
		}
		if row == nil {
			return tcErrMsg{fmt.Errorf("testcase %s not found", id)}
		}
		return tcLoadedMsg{id: id, tc: *row}
	}
}

func fetchStickieCmd(id string) tea.Cmd {
	return func() tea.Msg {
		cfg, err := cfgpkg.Load()
		if err != nil {
			return tcErrMsg{err}
		}
		ctx := contextForShort()
		defer ctx.cancel()
		db, err := pgdao.OpenApp(ctx.ctx, cfg)
		if err != nil {
			return tcErrMsg{err}
		}
		defer db.Close()
		st, err := pgdao.GetStickieByID(ctx.ctx, db, id)
		if err != nil {
			return tcErrMsg{err}
		}
		if st == nil {
			return tcErrMsg{fmt.Errorf("stickie %s not found", id)}
		}
		return stickLoadedMsg{id: id, s: *st}
	}
}

// detectAndLoadByIDCmd tries testcase first, then stickie, and returns a quickAddResult
func detectAndLoadByIDCmd(id string) tea.Cmd {
	return func() tea.Msg {
		cfg, err := cfgpkg.Load()
		if err != nil {
			return tcErrMsg{err}
		}
		ctx := contextForShort()
		defer ctx.cancel()
		db, err := pgdao.OpenApp(ctx.ctx, cfg)
		if err != nil {
			return tcErrMsg{err}
		}
		defer db.Close()
		if tc, err := pgdao.GetTestcaseByID(ctx.ctx, db, id); err == nil && tc != nil {
			return quickAddResult{id: id, kind: KindTestcase, tc: tc}
		}
		if st, err := pgdao.GetStickieByID(ctx.ctx, db, id); err == nil && st != nil {
			return quickAddResult{id: id, kind: KindStickie, st: st}
		}
		return quickAddResult{id: id, kind: ""}
	}
}

// small helper to provide a cancellable context with a default timeout
type shortCtx struct {
	ctx    context.Context
	cancel context.CancelFunc
}

func contextForShort() shortCtx {
	// 10s default to match other admin TUIs
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	return shortCtx{ctx: ctx, cancel: cancel}
}
