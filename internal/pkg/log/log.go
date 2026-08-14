// Package log provides the small amount of leveled logging the generator needs.
package log

import (
	"fmt"
	"io"
	"os"
)

var (
	out     io.Writer = os.Stderr
	verbose bool
)

// SetOutput redirects log output. Passing nil restores os.Stderr.
func SetOutput(w io.Writer) {
	if w == nil {
		w = os.Stderr
	}
	out = w
}

// SetVerbose enables or disables Debug output.
func SetVerbose(v bool) { verbose = v }

// Debug writes a message only when verbose logging is enabled.
func Debug(format string, args ...any) {
	if verbose {
		write("debug", format, args...)
	}
}

// Info writes an informational message.
func Info(format string, args ...any) { write("info", format, args...) }

// Error writes an error message.
func Error(format string, args ...any) { write("error", format, args...) }

func write(level, format string, args ...any) {
	fmt.Fprintf(out, "%-5s %s\n", level, fmt.Sprintf(format, args...))
}
