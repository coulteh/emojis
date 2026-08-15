// Package generator turns Unicode's published emoji data into a Go package
// whose functions return each emoji by name.
package generator

import (
	"fmt"
	"log/slog"
	"os"
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
	defer src.Close()

	ds, err := Parse(src, SourceURL)
	if err != nil {
		return err
	}

	model, err := Build(ds)
	if err != nil {
		return err
	}

	files, err := render(model, pkg)
	if err != nil {
		return err
	}

	for _, f := range files {
		if err := os.WriteFile(f.Name, f.Contents, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", f.Name, err)
		}
		slog.Info("wrote file", "path", f.Name, "bytes", len(f.Contents))
	}
	return nil
}
