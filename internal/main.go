// Command generator generates the emojis package from Unicode's emoji data.
//
// Run it with "go generate ./..." from the module root.
package main

import (
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"

	"emojis/internal/generator"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		slog.Error("generation failed", "err", err)
		os.Exit(1)
	}
}

// run parses args, which are the arguments after the program name, and
// generates the package. Asking for usage is not an error.
func run(args []string) error {
	cfg := generator.DefaultConfig()
	var verbose bool

	fs := flag.NewFlagSet("generator", flag.ContinueOnError)
	fs.StringVar(&cfg.Source, "source", cfg.Source, "URL or path of emoji-test.txt")
	fs.StringVar(&cfg.OutDir, "out", cfg.OutDir, "directory to write the generated package into")
	fs.StringVar(&cfg.Package, "package", cfg.Package, "package clause for the generated files")
	fs.BoolVar(&verbose, "verbose", false, "log progress in detail")
	fs.Usage = func() {
		fmt.Fprint(fs.Output(),
			"Usage: generator [flags]\n\n"+
				"Generate the emojis package from Unicode emoji data.\n\n"+
				"Reads emoji-test.txt, from Unicode by default or from a local path,\n"+
				"and writes the generated *_gen.go files.\n\n"+
				"Flags:\n")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil // Parse has already printed the usage.
		}
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected argument %q", fs.Arg(0))
	}

	level := slog.LevelInfo
	if verbose {
		level = slog.LevelDebug
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})))

	return generator.Generate(cfg)
}
