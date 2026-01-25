package blackboard

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"

	pgdao "github.com/flarebyte/baldrick-rebec/internal/dao/postgres"
)

// --- Blackboard helpers ---

func nullOrString(ns sql.NullString) string {
	if ns.Valid {
		return ns.String
	}
	return ""
}

// wrapIfValid mirrors exporter behavior for prose fields (wrap at 80 columns)
func wrapIfValid(ns sql.NullString) string {
	if ns.Valid {
		return wrapAt(ns.String, 80)
	}
	return ""
}

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
func derefLS(p *LiteralString) string {
	if p == nil {
		return ""
	}
	return string(*p)
}
func quoteShort(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 80 {
		s = s[:77] + "..."
	}
	return fmt.Sprintf("%q", s)
}

// --- Stickie field diffs ---

// Concise list of changed fields for a stickie
func listChangedStickieFields(r pgdao.Stickie, l stickieYAML) []string {
	out := make([]string, 0, 8)
	add := func(name string, diff bool) {
		if diff {
			out = append(out, name)
		}
	}
	add("name", strings.TrimSpace(getNS(r.Name)) != strings.TrimSpace(getStrPtr(l.Name)))
	add("archived", r.Archived != l.Archived)
	// exporter wraps notes at 80; normalize remote with wrapAt to avoid false diffs
	add("note", norm(wrapIfValid(r.Note)) != norm(getLitPtr(l.Note)))
	add("code", norm(getNS(r.Code)) != norm(getStrPtr(l.Code)))
	add("labels", !equalStringSets(r.Labels, l.Labels))
	add("priority_level", strings.TrimSpace(getNS(r.PriorityLevel)) != strings.TrimSpace(getStrPtr(l.Priority)))
	add("created_by_task_id", strings.TrimSpace(getNS(r.CreatedByTaskID)) != strings.TrimSpace(getStrPtr(l.CreatedByTask)))
	if floatChanged(r.Score, l.Score) {
		out = append(out, "score")
	}
	return out
}

type fieldDetail struct {
	name   string
	remote string
	local  string
}

func computeStickieFieldDiff(r pgdao.Stickie, l stickieYAML) []fieldDetail {
	det := make([]fieldDetail, 0, 8)
	add := func(name, rv, lv string, changed bool) {
		if changed {
			det = append(det, fieldDetail{name, rv, lv})
		}
	}
	add("name", getNS(r.Name), getStrPtr(l.Name), strings.TrimSpace(getNS(r.Name)) != strings.TrimSpace(getStrPtr(l.Name)))
	if r.Archived != l.Archived {
		add("archived", fmt.Sprintf("%v", r.Archived), fmt.Sprintf("%v", l.Archived), true)
	}
	if norm(wrapIfValid(r.Note)) != norm(getLitPtr(l.Note)) {
		add("note", shortHash(wrapIfValid(r.Note)), shortHash(getLitPtr(l.Note)), true)
	}
	if norm(getNS(r.Code)) != norm(getStrPtr(l.Code)) {
		add("code", shortHash(getNS(r.Code)), shortHash(getStrPtr(l.Code)), true)
	}
	if !equalStringSets(r.Labels, l.Labels) {
		add("labels", fmt.Sprintf("%v", sortedCopy(r.Labels)), fmt.Sprintf("%v", sortedCopy(l.Labels)), true)
	}
	add("priority_level", getNS(r.PriorityLevel), getStrPtr(l.Priority), strings.TrimSpace(getNS(r.PriorityLevel)) != strings.TrimSpace(getStrPtr(l.Priority)))
	add("created_by_task_id", getNS(r.CreatedByTaskID), getStrPtr(l.CreatedByTask), strings.TrimSpace(getNS(r.CreatedByTaskID)) != strings.TrimSpace(getStrPtr(l.CreatedByTask)))
	if floatChanged(r.Score, l.Score) {
		rv := ""
		lv := ""
		if r.Score.Valid {
			rv = fmt.Sprintf("%v", r.Score.Float64)
		} else {
			rv = "(nil)"
		}
		if l.Score != nil {
			lv = fmt.Sprintf("%v", *l.Score)
		} else {
			lv = "(nil)"
		}
		add("score", rv, lv, true)
	}
	return det
}

func formatDetailedFieldDiff(fields []fieldDetail) string {
	if len(fields) == 0 {
		return ""
	}
	var b strings.Builder
	for _, f := range fields {
		b.WriteString(" ")
		b.WriteString(f.name)
		b.WriteString("[")
		b.WriteString("remote=")
		b.WriteString(quoteShort(f.remote))
		b.WriteString(" local=")
		b.WriteString(quoteShort(f.local))
		b.WriteString("]")
	}
	return b.String()
}

// --- Small utils ---

func getNS(ns sql.NullString) string {
	if ns.Valid {
		return ns.String
	}
	return ""
}
func getStrPtr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
func getLitPtr(p *LiteralString) string {
	if p == nil {
		return ""
	}
	return string(*p)
}
func norm(s string) string { return strings.TrimSpace(s) }
func floatChanged(r sql.NullFloat64, lp *float64) bool {
	if r.Valid && lp != nil {
		return r.Float64 != *lp
	}
	if r.Valid != (lp != nil) {
		return true
	}
	return false
}
func equalStringSets(a, b []string) bool {
	sa := sortedCopy(a)
	sb := sortedCopy(b)
	if len(sa) != len(sb) {
		return false
	}
	for i := range sa {
		if sa[i] != sb[i] {
			return false
		}
	}
	return true
}
func sortedCopy(in []string) []string {
	cp := append([]string(nil), in...)
	sort.Strings(cp)
	return cp
}

func shortHash(s string) string {
	if s == "" {
		return ""
	}
	// reuse hashMaterial by wrapping into the same hashing
	h := hashMaterial(struct {
		S string `json:"s"`
	}{S: s})
	if len(h) > 8 {
		return h[:8]
	}
	return h
}
