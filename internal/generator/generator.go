// Package generator turns Unicode's published emoji data into a Go package
// whose functions return each emoji by name.
package generator

import (
	"bytes"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// pkg is the package clause the generated files carry.
const pkg = "emojis"

// Generate fetches the emoji data and writes the generated package into the
// working directory. Under "go generate" that is the directory holding the
// directive, which is the module root: where the emojis package lives.
func Generate() error {
	src, err := fetch()
	if err != nil {
		return err
	}
	// Nothing is read from it after this, so a failure to close has nothing
	// left to affect.
	defer func() { _ = src.Close() }()

	ds, err := Parse(src, SourceURL)
	if err != nil {
		return err
	}

	model, err := Build(ds)
	if err != nil {
		return err
	}

	// Renames found against the last generated package join the ones already
	// recorded, so an old name keeps working however many releases ago
	// Unicode replaced it.
	previous, err := readPrevious(".")
	if err != nil {
		return err
	}
	renames, err := loadRenames(renamesFile)
	if err != nil {
		return err
	}
	detected := detectRenames(previous, model, ds.Version)
	renames = model.applyRenames(append(renames, detected...))
	reportRenames(detected, model)

	files, err := render(model, pkg)
	if err != nil {
		return err
	}
	files = append(files, file{Name: renamesFile, Contents: formatRenames(renames)})

	// Read what is there before replacing it, so a scheduled run that finds
	// Unicode has published nothing new says so rather than looking like it
	// did work.
	reportFunctionChanges(".", model)

	changed, err := writeFiles(".", files)
	if err != nil {
		return err
	}
	if changed == 0 {
		slog.Info("no changes", "unicode", ds.Version, "functions", len(model.Bases))
	} else {
		slog.Info("regenerated", "unicode", ds.Version,
			"files_changed", changed, "functions", len(model.Bases))
	}
	return nil
}

// writeFiles writes each file whose contents differ from what is already on
// disk, and returns how many it changed. Leaving the identical ones alone
// keeps a no-op run from touching timestamps, and lets the caller tell a
// release with new emoji from one without.
func writeFiles(dir string, files []file) (int, error) {
	changed := 0
	for _, f := range files {
		path := filepath.Join(dir, f.Name)
		if existing, err := os.ReadFile(path); err == nil && bytes.Equal(existing, f.Contents) {
			slog.Info("file unchanged", "path", f.Name, "bytes", len(f.Contents))
			continue
		}
		if err := os.WriteFile(path, f.Contents, 0o644); err != nil {
			return changed, fmt.Errorf("write %s: %w", path, err)
		}
		slog.Info("wrote file", "path", f.Name, "bytes", len(f.Contents))
		changed++
	}
	return changed, nil
}

// emojiFile is the generated file holding one function per emoji.
const emojiFile = "emoji_gen.go"

// funcRe matches the exported functions in a previously generated emoji file.
var funcRe = regexp.MustCompile(`(?m)^func ([A-Z]\w*)\(`)

// listedNames is how many names a report spells out before summarising.
const listedNames = 10

// reportFunctionChanges says which emoji a new release added or took away,
// by reading back the file about to be replaced.
//
// Removals are worth a warning: the generated functions are this package's
// public API. A rename keeps its old function as an alias, so what is left is
// an emoji Unicode has withdrawn, or a rename the generator failed to spot.
func reportFunctionChanges(dir string, m *Model) {
	previous, err := os.ReadFile(filepath.Join(dir, emojiFile))
	if err != nil {
		slog.Debug("nothing to compare against", "path", emojiFile, "err", err)
		return
	}

	added, removed := changedFunctions(previous, m)
	if len(added) == 0 && len(removed) == 0 {
		slog.Info("same emoji as last time", "functions", len(m.Bases))
		return
	}
	if len(added) > 0 {
		slog.Info("emoji added", "count", len(added), "names", summarise(added))
	}
	if len(removed) > 0 {
		slog.Warn("emoji removed; these were exported functions",
			"count", len(removed), "names", summarise(removed))
	}
}

// changedFunctions compares the functions declared in a previously generated
// emoji file with the ones about to replace them.
func changedFunctions(previous []byte, m *Model) (added, removed []string) {
	before := make(map[string]bool)
	for _, match := range funcRe.FindAllStringSubmatch(string(previous), -1) {
		before[match[1]] = true
	}
	after := make(map[string]bool, len(m.Bases)+len(m.Aliases))
	for _, b := range m.Bases {
		after[b.Ident] = true
	}
	for _, a := range m.Aliases {
		after[a.Ident] = true
	}
	return only(after, before), only(before, after)
}

// only returns the sorted keys of a that b does not have.
func only(a, b map[string]bool) []string {
	var out []string
	for k := range a {
		if !b[k] {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

// summarise keeps a long list of names to one readable line.
func summarise(names []string) string {
	if len(names) <= listedNames {
		return strings.Join(names, ", ")
	}
	return fmt.Sprintf("%s and %d more",
		strings.Join(names[:listedNames], ", "), len(names)-listedNames)
}
