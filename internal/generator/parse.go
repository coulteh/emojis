package generator

import (
	"bufio"
	"fmt"
	"io"
	"log/slog"
	"regexp"
	"strconv"
	"strings"
)

// Emoji is a single row of emoji-test.txt.
type Emoji struct {
	Name     string // CLDR short name, e.g. "thumbs up: dark skin tone"
	Sequence string // the emoji itself
	Group    string // e.g. "People & Body"
	Subgroup string // e.g. "hand-fingers-closed"
}

// Dataset is everything parsed out of one emoji-test.txt.
type Dataset struct {
	Source  string
	Version string // Unicode emoji version, e.g. "17.0"
	Emojis  []Emoji
}

var (
	// "😀 E1.0 grinning face", or "😀 grinning face" without the emoji version,
	// which Unicode only started stamping on each line in emoji 12.1. Nothing
	// here reads that field, so it is optional rather than required.
	commentRe  = regexp.MustCompile(`^(\S+)\s+(?:E\d+(?:\.\d+)?\s+)?(.+)$`)
	versionRe  = regexp.MustCompile(`^#\s*Version:\s*(\S+)`)
	groupRe    = regexp.MustCompile(`^#\s*group:\s*(.+)$`)
	subgroupRe = regexp.MustCompile(`^#\s*subgroup:\s*(.+)$`)
)

// statusQualified is the only status that yields a callable emoji. The
// minimally-qualified and unqualified rows are the same emoji missing
// presentation selectors, and the component rows are the bare skin tone and
// hair modifiers, which are not emoji in their own right.
const statusQualified = "fully-qualified"

// Parse reads emoji-test.txt.
func Parse(r io.Reader, source string) (*Dataset, error) {
	ds := &Dataset{Source: source}

	var group, subgroup string
	var skipped int

	scanner := bufio.NewScanner(r)
	for line := 1; scanner.Scan(); line++ {
		text := scanner.Text()

		if strings.HasPrefix(text, "#") {
			if m := versionRe.FindStringSubmatch(text); m != nil && ds.Version == "" {
				ds.Version = m[1]
			}
			if m := groupRe.FindStringSubmatch(text); m != nil {
				group, subgroup = m[1], ""
			}
			if m := subgroupRe.FindStringSubmatch(text); m != nil {
				subgroup = m[1]
			}
			continue
		}
		if strings.TrimSpace(text) == "" {
			continue
		}

		emoji, status, err := parseLine(text)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", line, err)
		}
		if status != statusQualified {
			skipped++
			continue
		}
		if group == "" {
			return nil, fmt.Errorf("line %d: emoji %q appears before any group header", line, emoji.Name)
		}
		emoji.Group, emoji.Subgroup = group, subgroup
		ds.Emojis = append(ds.Emojis, emoji)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read %s: %w", source, err)
	}

	if len(ds.Emojis) == 0 {
		return nil, fmt.Errorf("no %s emoji found in %s", statusQualified, source)
	}
	if ds.Version == "" {
		return nil, fmt.Errorf("no version header found in %s", source)
	}
	slog.Info("parsed emoji data",
		"emoji", len(ds.Emojis), "unicode", ds.Version, "skipped", skipped)
	return ds, nil
}

// parseLine parses a data row:
//
//	1F44D 1F3FF ; fully-qualified # 👍🏿 E1.0 thumbs up: dark skin tone
func parseLine(text string) (Emoji, string, error) {
	code, rest, ok := strings.Cut(text, ";")
	if !ok {
		return Emoji{}, "", fmt.Errorf("no %q separator in %q", ";", text)
	}
	status, comment, ok := strings.Cut(rest, "#")
	if !ok {
		return Emoji{}, "", fmt.Errorf("no %q separator in %q", "#", text)
	}

	m := commentRe.FindStringSubmatch(strings.TrimSpace(comment))
	if m == nil {
		return Emoji{}, "", fmt.Errorf("cannot parse comment %q", strings.TrimSpace(comment))
	}

	seq, err := decode(strings.TrimSpace(code))
	if err != nil {
		return Emoji{}, "", err
	}
	// The comment repeats the sequence; disagreement means we misread the row.
	if seq != m[1] {
		return Emoji{}, "", fmt.Errorf("code points %q decode to %q but comment shows %q",
			strings.TrimSpace(code), seq, m[1])
	}

	return Emoji{Name: strings.TrimSpace(m[2]), Sequence: seq}, strings.TrimSpace(status), nil
}

// decode turns "1F44D 1F3FF" into the string it represents.
func decode(code string) (string, error) {
	var sb strings.Builder
	for _, field := range strings.Fields(code) {
		cp, err := strconv.ParseUint(field, 16, 32)
		if err != nil {
			return "", fmt.Errorf("bad code point %q: %w", field, err)
		}
		sb.WriteRune(rune(cp))
	}
	return sb.String(), nil
}
