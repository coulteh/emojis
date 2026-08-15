package generator

import (
	"log/slog"
	"sort"
	"strings"
)

// A Span is a half-open range of a blob, [Off, End).
type Span struct{ Off, End int }

// A Blob is many strings packed into one, so that the generated package holds a
// single string constant instead of thousands of separate ones. Each of those
// would otherwise cost a string header in the binary on top of its bytes.
//
// Strings that already appear inside the blob are not appended again, which is
// worth more than it sounds: every skin tone form contains the emoji it is a
// form of, so "thumbs up" costs nothing once "thumbs up: light skin tone" is
// there. The same holds for names, where "man" is inside "woman".
type Blob struct {
	Text  string
	spans map[string]Span
}

// Span returns where s sits in the blob. Strings not passed to NewBlob return
// the zero Span, which is the empty string.
func (b *Blob) Span(s string) Span { return b.spans[s] }

// NewBlob packs items into a single string.
//
// Longest first, so that a string is packed before anything it contains and
// the shorter one is then found inside it rather than appended. Ties break
// lexicographically to keep the output identical from one run to the next.
func NewBlob(name string, items []string) *Blob {
	uniq := make([]string, 0, len(items))
	seen := make(map[string]bool, len(items))
	total := 0
	for _, s := range items {
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		total += len(s)
		uniq = append(uniq, s)
	}
	sort.Slice(uniq, func(i, j int) bool {
		if len(uniq[i]) != len(uniq[j]) {
			return len(uniq[i]) > len(uniq[j])
		}
		return uniq[i] < uniq[j]
	})

	b := &Blob{spans: make(map[string]Span, len(uniq))}
	var sb strings.Builder
	for _, s := range uniq {
		// Builder.String does not copy, so this stays cheap.
		if at := strings.Index(sb.String(), s); at >= 0 {
			b.spans[s] = Span{at, at + len(s)}
			continue
		}
		off := sb.Len()
		sb.WriteString(s)
		b.spans[s] = Span{off, off + len(s)}
	}
	b.Text = sb.String()

	slog.Info("packed blob", "kind", name, "items", len(uniq), "bytes", len(b.Text), "unpacked", total)
	return b
}
