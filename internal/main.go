// Command internal generates the emojis package from Unicode's emoji data.
//
// Run it with "go generate ./..." from the module root.
package main

import (
	"os"

	"github.com/spf13/cobra"

	"emojis/internal/pkg/generator"
	"emojis/internal/pkg/log"
)

func main() {
	cfg := generator.DefaultConfig()
	var verbose bool

	cmd := &cobra.Command{
		Use:   "generator",
		Short: "Generate the emojis package from Unicode emoji data",
		Long: "Generate the emojis package from Unicode emoji data.\n\n" +
			"Reads emoji-test.txt, from Unicode by default or from a local path,\n" +
			"and writes the generated *_gen.go files.",
		SilenceUsage: true,
		RunE: func(*cobra.Command, []string) error {
			log.SetVerbose(verbose)
			return generator.Generate(cfg)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&cfg.Source, "source", "s", cfg.Source, "URL or path of emoji-test.txt")
	flags.StringVarP(&cfg.OutDir, "out", "o", cfg.OutDir, "directory to write the generated package into")
	flags.StringVarP(&cfg.Package, "package", "p", cfg.Package, "package clause for the generated files")
	flags.BoolVarP(&verbose, "verbose", "v", false, "log progress in detail")

	if err := cmd.Execute(); err != nil {
		log.Error("%v", err)
		os.Exit(1)
	}
}
