// Command generator generates the emojis package from Unicode's emoji data.
//
// It takes no arguments beyond -verbose: what it reads, what it writes and
// where all follow from this being the emojis repository. Run it with
// "go generate ./..." from the module root.
package main

import (
	"flag"
	"log/slog"
	"os"

	"emojis/internal/generator"
)

func main() {
	verbose := flag.Bool("verbose", false, "log progress in detail")
	flag.Parse()

	level := slog.LevelInfo
	if *verbose {
		level = slog.LevelDebug
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})))

	if err := generator.Generate(); err != nil {
		slog.Error("generation failed", "err", err)
		os.Exit(1)
	}
}
