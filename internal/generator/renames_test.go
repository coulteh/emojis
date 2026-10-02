package generator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func buildSample(t *testing.T) *Model {
	t.Helper()
	m, err := Build(parseSample(t))
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	return m
}

// writeRendered generates the package for m into a fresh directory.
func writeRendered(t *testing.T, m *Model) string {
	t.Helper()
	files, err := render(m, "emojis")
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	dir := t.TempDir()
	if _, err := writeFiles(dir, files); err != nil {
		t.Fatalf("writeFiles: %v", err)
	}
	return dir
}

func TestReadPreviousRoundTrips(t *testing.T) {
	m := buildSample(t)
	prev, err := readPrevious(writeRendered(t, m))
	if err != nil {
		t.Fatalf("readPrevious: %v", err)
	}
	if len(prev) != len(m.Bases) {
		t.Errorf("read back %d names, want %d", len(prev), len(m.Bases))
	}
	for _, b := range m.Bases {
		if got := prev[b.Name]; got != b.Plain {
			t.Errorf("read back %q as %q, want %q", b.Name, got, b.Plain)
		}
	}
}

func TestReadPreviousFallsBackToAModifiedForm(t *testing.T) {
	m := buildSample(t)
	// An emoji Unicode defines only with modifiers has nothing in baseEmoji.
	for _, b := range m.Bases {
		if b.Name == "thumbs up" {
			b.Plain = ""
		}
	}
	prev, err := readPrevious(writeRendered(t, m))
	if err != nil {
		t.Fatalf("readPrevious: %v", err)
	}
	if got, want := prev["thumbs up"], "\U0001F44D\U0001F3FB"; got != want {
		t.Errorf("read back a modified-only emoji as %q, want its first form %q", got, want)
	}
}

func TestReadPreviousWithNothingToRead(t *testing.T) {
	prev, err := readPrevious(t.TempDir())
	if prev != nil || err != nil {
		t.Errorf("readPrevious of an empty directory = %v, %v; want nothing and no error", prev, err)
	}
}

// Generating without being able to read the last release back would retire
// every renamed function without a word, so a mangled file is an error.
func TestReadPreviousRejectsMangledTables(t *testing.T) {
	for name, src := range map[string]string{
		"not Go":         "this is not Go",
		"missing tables": "package emojis\n\nconst emojiBlob = \"\"\n",
		"span past the blob": "package emojis\n\nconst emojiBlob = \"\"\nconst nameBlob = \"ab\"\n" +
			"var baseNames = [1]span{{0, 9}}\nvar baseEmoji = [1]span{{0, 0}}\n" +
			"var styledKeys = [0]uint32{}\nvar styledEmoji = [0]span{}\n",
	} {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, tablesFile), []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := readPrevious(dir); err == nil {
			t.Errorf("%s: readPrevious succeeded, want an error", name)
		}
	}
}

func TestDetectRenames(t *testing.T) {
	m := buildSample(t)
	previous := map[string]string{
		"grinning face":   "\U0001F600", // unchanged
		"thumbs up":       "\U0001F44D", // unchanged
		"grinning smiley": "\U0001F600", // the same emoji under another name
		"human":           "\U0001F9D1\U0001F3FD",
		"retired emoji":   "\U0001F9FF", // gone altogether: a removal, not a rename
	}
	got := detectRenames(previous, m, "18.0")
	want := []Rename{
		{Old: "grinning smiley", New: "grinning face", Version: "18.0"},
		{Old: "human", New: "person", Version: "18.0"},
	}
	if len(got) != len(want) {
		t.Fatalf("detectRenames = %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("rename %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestApplyRenames(t *testing.T) {
	m := buildSample(t)
	kept := m.applyRenames([]Rename{
		{Old: "smiley", New: "grinning smiley", Version: "17.0"},
		{Old: "grinning smiley", New: "grinning face", Version: "18.0"},
		{Old: "human", New: "person", Version: "18.0"},
		{Old: "withdrawn", New: "no longer published", Version: "18.0"},
		{Old: "thumbs up", New: "grinning face", Version: "18.0"},
	})

	want := map[string]string{
		"grinning smiley": "grinning face",
		"human":           "person",
		// Renamed twice: it goes straight to the current name.
		"smiley": "grinning face",
	}
	if len(kept) != len(want) {
		t.Fatalf("kept %+v, want renames of %v", kept, want)
	}
	for _, r := range kept {
		if want[r.Old] != r.New {
			t.Errorf("kept %q -> %q, want -> %q", r.Old, r.New, want[r.Old])
		}
	}
	if kept[2].Version != "17.0" {
		t.Errorf("following a chain changed when %q was renamed to %q", kept[2].Old, kept[2].Version)
	}

	idents := map[string]string{}
	for _, a := range m.Aliases {
		idents[a.Ident] = a.Current.Ident
	}
	for alias, current := range map[string]string{
		"GrinningSmiley": "GrinningFace", "Human": "Person", "Smiley": "GrinningFace",
	} {
		if idents[alias] != current {
			t.Errorf("alias %s calls %q, want %s", alias, idents[alias], current)
		}
	}
}

func TestRenderedAliases(t *testing.T) {
	m := buildSample(t)
	m.applyRenames([]Rename{
		{Old: "grinning smiley", New: "grinning face", Version: "18.0"},
		{Old: "human", New: "person", Version: "18.0"},
	})
	files, err := render(m, "emojis")
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	emoji, tables := string(files[2].Contents), string(files[1].Contents)

	for _, want := range []string{
		"func GrinningSmiley() string { return GrinningFace() }",
		// It keeps taking variants if the emoji does.
		"func Human(v ...Variant) string { return Person(v...) }",
		"// Deprecated: Use GrinningFace.",
		"until emoji 18.0 and now calls \"grinning face\"",
	} {
		if !strings.Contains(emoji, want) {
			t.Errorf("emoji_gen.go does not contain %q", want)
		}
	}
	if !strings.Contains(tables, "var renamedNames = [2]span{") || !strings.Contains(tables, "var renamedRows = [2]uint16{") {
		t.Errorf("tables_gen.go is missing the renamed-name tables")
	}

	// The old names are not current names, so they stay out of what a later
	// run reads back as the previous release.
	dir := writeRendered(t, m)
	prev, err := readPrevious(dir)
	if err != nil {
		t.Fatalf("readPrevious: %v", err)
	}
	if _, ok := prev["grinning smiley"]; ok || len(prev) != len(m.Bases) {
		t.Errorf("read back %d names including old ones, want the %d current names", len(prev), len(m.Bases))
	}

	// An alias still declares its function, so it is not reported removed.
	if _, removed := changedFunctions(files[2].Contents, m); len(removed) > 0 {
		t.Errorf("regenerating with aliases reported %v removed", removed)
	}
}

func TestRenamesFileRoundTrips(t *testing.T) {
	renames := []Rename{
		{Old: "flag: St. Helena", New: "flag: St. Helena, Ascension & Tristan da Cunha", Version: "18.0"},
		{Old: "flag: Heard & McDonald Islands", New: "flag: Heard Island & McDonald Islands", Version: "18.0"},
	}
	src := formatRenames(renames)
	got, err := parseRenames(src, "renames.txt")
	if err != nil {
		t.Fatalf("parseRenames: %v\n%s", err, src)
	}
	// Written sorted by old name.
	if len(got) != 2 || got[0] != renames[1] || got[1] != renames[0] {
		t.Errorf("round trip gave %+v", got)
	}
	if !strings.HasPrefix(string(src), "# ") {
		t.Errorf("renames file does not start with its explanatory header:\n%s", src)
	}
}

func TestParseRenamesRejectsBadLines(t *testing.T) {
	for _, src := range []string{
		"old ; new\n",
		"old ; new ; 18.0 ; extra\n",
		"old ;  ; 18.0\n",
		"old ; new ; 18.0\nold ; newer ; 19.0\n",
	} {
		if _, err := parseRenames([]byte(src), "renames.txt"); err == nil {
			t.Errorf("parseRenames(%q) succeeded, want an error", src)
		}
	}
}

func TestLoadRenamesWithNoFile(t *testing.T) {
	got, err := loadRenames(filepath.Join(t.TempDir(), "renames.txt"))
	if got != nil || err != nil {
		t.Errorf("loadRenames of a missing file = %v, %v; want nothing and no error", got, err)
	}
}
