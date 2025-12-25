package prompt

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

// Styles
var (
	pStyleHeader  = lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Bold(true)  // cyan
	pStyleLabel   = lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Bold(true)   // gray
	pStyleValue   = lipgloss.NewStyle().Foreground(lipgloss.Color("7"))              // white-ish
	pStyleHelp    = lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Italic(true) // dim italic
	pStyleCursor  = lipgloss.NewStyle().Foreground(lipgloss.Color("13")).Bold(true)  // magenta
	pStyleDivider = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))              // gray line
	pStyleWarn    = lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Bold(true)   // red
)

// BlockKind enumerates allowed kinds for blocks
type BlockKind string

const (
	KindH1       BlockKind = "h1"
	KindBody     BlockKind = "body"
	KindTestcase BlockKind = "testcase"
	KindStickie  BlockKind = "stickie"
)

var allKinds = []BlockKind{KindH1, KindBody, KindTestcase, KindStickie}

// DesignBlock represents a prompt designer block
type DesignBlock struct {
	ID       string   `json:"id"`
	Kind     string   `json:"kind"`
	Value    string   `json:"value"`
	Scripts  []string `json:"scripts"`
	Disabled bool     `json:"disabled"`
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
	blocks   []DesignBlock
	cursor   int // which block
	quitting bool
	export   bool // if true, print JSON on exit

	// detail pane selection 0..4 (id, kind, value, scripts, disabled)
	detailIdx int
	// inline text editing state
	editing    bool
	editBuffer string
	// which field is being edited (same as detailIdx when editing text)
}

func newPromptModel(initial []DesignBlock) promptModel {
	if len(initial) == 0 {
		initial = []DesignBlock{newDefaultBlock()}
	}
	return promptModel{blocks: initial, cursor: 0, detailIdx: 0}
}

func newDefaultBlock() DesignBlock {
	return DesignBlock{
		ID:       uuid.NewString(),
		Kind:     string(KindH1),
		Value:    "",
		Scripts:  []string{},
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
				// Commit edit
				m = m.commitEdit()
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
			case tea.KeyRunes:
				if len(msg.Runes) > 0 {
					m.editBuffer += string(msg.Runes)
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
		case "tab":
			m.detailIdx = (m.detailIdx + 1) % 5
			return m, nil
		case "shift+tab":
			m.detailIdx = (m.detailIdx + 4) % 5
			return m, nil
		case "a":
			// Add after current
			nb := newDefaultBlock()
			if len(m.blocks) == 0 || m.cursor >= len(m.blocks)-1 {
				m.blocks = append(m.blocks, nb)
				m.cursor = len(m.blocks) - 1
			} else {
				idx := m.cursor + 1
				m.blocks = append(m.blocks[:idx], append([]DesignBlock{nb}, m.blocks[idx:]...)...)
				m.cursor = idx
			}
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
		case "enter", "e":
			// Begin editing selected text field (id/value/scripts). Kind/bool handled via keys
			if len(m.blocks) == 0 {
				return m, nil
			}
			switch m.detailIdx {
			case 0: // id
				m.editing = true
				m.editBuffer = m.blocks[m.cursor].ID
			case 2: // value
				m.editing = true
				m.editBuffer = m.blocks[m.cursor].Value
			case 3: // scripts (CSV)
				m.editing = true
				m.editBuffer = strings.Join(m.blocks[m.cursor].Scripts, ", ")
			default:
				// no-op; for kind/bool use dedicated keys
			}
			return m, nil
		case "x":
			// Toggle disable
			if len(m.blocks) > 0 {
				m.blocks[m.cursor].Disabled = !m.blocks[m.cursor].Disabled
			}
			return m, nil
		case "K", "t":
			// Cycle kind forward
			if len(m.blocks) > 0 {
				m.blocks[m.cursor].Kind = string(nextKind(BlockKind(m.blocks[m.cursor].Kind)))
			}
			return m, nil
		}
	}
	return m, nil
}

func (m promptModel) View() string {
	var b strings.Builder
	b.WriteString(pStyleHeader.Render("Prompt Designer") + "\n")
	b.WriteString(pStyleHelp.Render("Keys: ↑/k, ↓/j, a=add, d=del, tab/shift+tab=field, e/enter=edit, t/K=kind, x=disable, s=save JSON, q") + "\n")
	b.WriteString(pStyleDivider.Render(strings.Repeat("─", 60)) + "\n")

	// List of blocks
	if len(m.blocks) == 0 {
		b.WriteString("No blocks. Press 'a' to add.\n")
		return b.String()
	}
	for i, blk := range m.blocks {
		cursor := "  "
		if i == m.cursor {
			cursor = pStyleCursor.Render("> ")
		}
		title := summarizeBlock(blk)
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

		b.WriteString(renderField(0, "ID: ", blk.ID) + "\n")
		b.WriteString(renderField(1, "Kind: ", blk.Kind) + "\n")
		b.WriteString(renderField(2, "Value: ", blk.Value) + "\n")
		b.WriteString(renderField(3, "Scripts: ", strings.Join(blk.Scripts, ", ")) + "\n")
		b.WriteString(renderField(4, "Disabled: ", fmt.Sprintf("%v", blk.Disabled)) + "\n")

		if m.editing {
			b.WriteString(pStyleHelp.Render("Editing: ") + pStyleValue.Render(m.editBuffer) + "\n")
			b.WriteString(pStyleHelp.Render("enter=apply, esc=cancel, ctrl+u=clear"))
			b.WriteString("\n")
		}
	}
	return b.String()
}

func summarizeBlock(b DesignBlock) string {
	switch strings.ToLower(strings.TrimSpace(b.Kind)) {
	case string(KindH1):
		if strings.TrimSpace(b.Value) == "" {
			return "h1: <title>"
		}
		return "h1: " + b.Value
	case string(KindBody):
		v := strings.TrimSpace(b.Value)
		if v == "" {
			return "body: <text>"
		}
		if len(v) > 40 {
			v = v[:40] + "…"
		}
		return "body: " + v
	case string(KindTestcase):
		if strings.TrimSpace(b.Value) == "" {
			return "testcase: <uuid>"
		}
		return "testcase: " + b.Value
	case string(KindStickie):
		if strings.TrimSpace(b.Value) == "" {
			return "stickie: <uuid>"
		}
		return "stickie: " + b.Value
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
	case 0: // id
		m.blocks[m.cursor].ID = strings.TrimSpace(m.editBuffer)
	case 2: // value
		m.blocks[m.cursor].Value = m.editBuffer
	case 3: // scripts CSV
		txt := strings.TrimSpace(m.editBuffer)
		if txt == "" {
			m.blocks[m.cursor].Scripts = []string{}
		} else {
			parts := strings.Split(txt, ",")
			out := make([]string, 0, len(parts))
			for _, p := range parts {
				s := strings.TrimSpace(p)
				if s != "" {
					out = append(out, s)
				}
			}
			m.blocks[m.cursor].Scripts = out
		}
	}
	m.editing = false
	m.editBuffer = ""
	return m
}
