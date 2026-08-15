// Package generator turns Unicode's published emoji data into a Go package
// whose functions return each emoji by name.
package generator

import (
	"fmt"
	"os"
	"path/filepath"

	"emojis/internal/pkg/log"
)

// Config controls a run of the generator.
type Config struct {
	Source  string // URL or file path of emoji-test.txt
	OutDir  string // directory to write the generated files into
	Package string // package clause for the generated files
}

// DefaultConfig generates the emojis package at the module root.
func DefaultConfig() Config {
	return Config{Source: DefaultSource, OutDir: ".", Package: "emojis"}
}

// Generate fetches the emoji data and writes the generated package.
func Generate(cfg Config) error {
	src, err := open(cfg.Source)
	if err != nil {
		return err
	}
	defer src.Close()

	ds, err := Parse(src, cfg.Source)
	if err != nil {
		return err
	}

	model, err := Build(ds)
	if err != nil {
		return err
	}

	files, err := render(model, cfg.Package)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(cfg.OutDir, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", cfg.OutDir, err)
	}
	for _, f := range files {
		path := filepath.Join(cfg.OutDir, f.Name)
		if err := os.WriteFile(path, f.Contents, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", path, err)
		}
		log.Info("wrote %s (%d bytes)", path, len(f.Contents))
	}
	return nil
}
