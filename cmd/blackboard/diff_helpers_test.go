package blackboard

import (
	"database/sql"
	"testing"

	pgdao "github.com/flarebyte/baldrick-rebec/internal/dao/postgres"
)

func contains(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}

func TestListChangedStickieFields_NoteWrappingNormalized(t *testing.T) {
	long := "This is a very long note that should be wrapped by the exporter at eighty characters per line, ensuring equality in diff when comparing remote DB and local YAML."
	wrapped := wrapAt(long, 80)

	r := pgdao.Stickie{Note: sql.NullString{Valid: true, String: long}}
	ly := stickieYAML{}
	ls := LiteralString(wrapped)
	ly.Note = &ls

	fields := listChangedStickieFields(r, ly)
	if len(fields) != 0 && contains(fields, "note") {
		t.Fatalf("expected no diff on note after wrapping; got %v", fields)
	}
}

func TestListChangedStickieFields_LabelsOrderIgnored(t *testing.T) {
	r := pgdao.Stickie{Labels: []string{"b", "a"}}
	ly := stickieYAML{Labels: []string{"a", "b"}}
	fields := listChangedStickieFields(r, ly)
	if contains(fields, "labels") {
		t.Fatalf("expected labels to be equal as sets; got %v", fields)
	}
}

func TestListChangedStickieFields_DetectsNameChange(t *testing.T) {
	r := pgdao.Stickie{Name: sql.NullString{Valid: true, String: "Foo"}}
	name := "Bar"
	ly := stickieYAML{Name: &name}
	fields := listChangedStickieFields(r, ly)
	if !contains(fields, "name") {
		t.Fatalf("expected name to be reported changed; got %v", fields)
	}
}

func TestComputeStickieFieldDiff_NoNoteDiffWhenWrapped(t *testing.T) {
	long := "Another long note that will be wrapped at eighty characters, keeping the representation stable between DB and YAML files."
	wrapped := wrapAt(long, 80)
	r := pgdao.Stickie{Note: sql.NullString{Valid: true, String: long}}
	ly := stickieYAML{}
	ls := LiteralString(wrapped)
	ly.Note = &ls

	det := computeStickieFieldDiff(r, ly)
	for _, f := range det {
		if f.name == "note" {
			t.Fatalf("expected no note diff after wrapping; got %+v", det)
		}
	}
}

func TestWrapIfValid_BlackboardProse(t *testing.T) {
	in := sql.NullString{Valid: true, String: "short text"}
	got := wrapIfValid(in)
	want := wrapAt(in.String, 80)
	if got != want {
		t.Fatalf("wrapIfValid mismatch: got %q want %q", got, want)
	}
}
