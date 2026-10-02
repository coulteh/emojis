package generator

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"sort"
	"strings"
)

// renamesFile records every emoji Unicode has renamed since this package began
// generating it. The path is relative to the module root, where the generator
// runs.
const renamesFile = "internal/generator/renames.txt"

// A Rename is an emoji whose CLDR name Unicode changed while leaving the emoji
// itself alone. Emoji 18.0 renamed "flag: St. Helena" to "flag: St. Helena,
// Ascension & Tristan da Cunha", for instance, when the territory's name
// changed.
//
// The generated functions are named after the CLDR name, so each rename would
// otherwise delete an exported function and break every caller of it, along
// with every Lookup of the old name, over an emoji that is still there.
type Rename struct {
	Old     string // the name Unicode used to give the emoji
	New     string // the name it gives it now
	Version string // the emoji release that renamed it
}

// An Alias is a rename that survives into the generated package: a deprecated
// function under the old name, and an entry letting Lookup find the old name.
type Alias struct {
	Ident   string
	Rename  Rename
	Current *Base
}

// renamesHeader opens the renames file. It is written by the generator, so
// this is regenerated along with the rest of the file.
const renamesHeader = `# Emoji Unicode has renamed since this package began generating them.
#
# The generator adds a line here when an emoji's name changes from one release
# to the next while the emoji itself stays the same. Each line keeps the old
# name working: as a deprecated function, and as a name Lookup accepts. Delete
# a line to retire that old name.
#
# old name ; current name ; emoji release that renamed it
`

// loadRenames reads the renames file. A missing file means nothing has been
// renamed yet.
func loadRenames(path string) ([]Rename, error) {
	src, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return parseRenames(src, path)
}

func parseRenames(src []byte, path string) ([]Rename, error) {
	var renames []Rename
	seen := make(map[string]bool)
	scanner := bufio.NewScanner(bytes.NewReader(src))
	for line := 1; scanner.Scan(); line++ {
		text := strings.TrimSpace(scanner.Text())
		if text == "" || strings.HasPrefix(text, "#") {
			continue
		}
		fields := strings.Split(text, ";")
		if len(fields) != 3 {
			return nil, fmt.Errorf("%s:%d: want \"old name ; current name ; release\", got %q", path, line, text)
		}
		r := Rename{
			Old:     strings.TrimSpace(fields[0]),
			New:     strings.TrimSpace(fields[1]),
			Version: strings.TrimSpace(fields[2]),
		}
		if r.Old == "" || r.New == "" || r.Version == "" {
			return nil, fmt.Errorf("%s:%d: empty field in %q", path, line, text)
		}
		if seen[r.Old] {
			return nil, fmt.Errorf("%s:%d: %q is renamed twice", path, line, r.Old)
		}
		seen[r.Old] = true
		renames = append(renames, r)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return renames, nil
}

// formatRenames writes the renames file, sorted by old name so it reads the
// same way every time it is regenerated.
func formatRenames(renames []Rename) []byte {
	sorted := append([]Rename(nil), renames...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Old < sorted[j].Old })

	var buf bytes.Buffer
	buf.WriteString(renamesHeader)
	for _, r := range sorted {
		fmt.Fprintf(&buf, "%s ; %s ; %s\n", r.Old, r.New, r.Version)
	}
	return buf.Bytes()
}

// detectRenames finds the emoji whose name the previous release used but this
// one does not, while the emoji itself is still here under another name.
// previous maps each name the last generated package held to its emoji.
//
// An emoji that has gone altogether is a removal rather than a rename, and is
// left for reportFunctionChanges to warn about.
func detectRenames(previous map[string]string, m *Model, version string) []Rename {
	current := make(map[string]bool, len(m.Bases))
	bySequence := make(map[string]*Base)
	for _, b := range m.Bases {
		current[b.Name] = true
		if b.Plain != "" {
			bySequence[b.Plain] = b
		}
		for _, f := range b.Forms {
			bySequence[f.Sequence] = b
		}
	}

	var renames []Rename
	for old, seq := range previous {
		if current[old] {
			continue
		}
		if b := bySequence[seq]; b != nil {
			renames = append(renames, Rename{Old: old, New: b.Name, Version: version})
		}
	}
	sort.Slice(renames, func(i, j int) bool { return renames[i].Old < renames[j].Old })
	return renames
}

// applyRenames turns the renames into aliases on the model and returns the
// renames still in force, which is what the renames file should now hold.
//
// A name renamed twice is pointed straight at the emoji's current name. A
// rename is dropped if its emoji has since gone, or if Unicode has given its
// old name back to an emoji, which then needs the name more than the alias.
func (m *Model) applyRenames(renames []Rename) []Rename {
	byName := make(map[string]*Base, len(m.Bases))
	for _, b := range m.Bases {
		byName[b.Name] = b
	}
	next := make(map[string]string, len(renames))
	for _, r := range renames {
		next[r.Old] = r.New
	}

	sorted := append([]Rename(nil), renames...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Old < sorted[j].Old })

	var kept []Rename
	for _, r := range sorted {
		if byName[r.Old] != nil {
			slog.Warn("dropping alias: Unicode uses the old name again", "name", r.Old)
			continue
		}
		// Each step follows a rename, so a chain longer than the list has
		// gone round in a circle.
		name := r.New
		for steps := 0; byName[name] == nil && next[name] != "" && steps < len(renames); steps++ {
			name = next[name]
		}
		current := byName[name]
		if current == nil {
			slog.Warn("dropping alias: the emoji is no longer published", "name", r.Old, "renamed_to", r.New)
			continue
		}
		r.New = name
		kept = append(kept, r)
		m.Aliases = append(m.Aliases, &Alias{Ident: m.idents.add(r.Old), Rename: r, Current: current})
	}
	return kept
}

// reportRenames logs the renames a release brought, by function name, which is
// what callers will notice.
func reportRenames(detected []Rename, m *Model) {
	brought := make(map[string]bool, len(detected))
	for _, r := range detected {
		brought[r.Old] = true
	}
	var names []string
	for _, a := range m.Aliases {
		if brought[a.Rename.Old] {
			names = append(names, a.Ident+" -> "+a.Current.Ident)
		}
	}
	if len(names) > 0 {
		slog.Info("emoji renamed; the old names stay as deprecated aliases",
			"count", len(names), "names", summarise(names))
	}
}
