package generator

import (
	"bytes"
	"embed"
	"fmt"
	"go/format"
	"sort"
	"strconv"
	"strings"
	"text/template"
)

//go:embed templates/*.go.tmpl
var templates embed.FS

var tmpl = template.Must(template.ParseFS(templates, "templates/*.go.tmpl"))

// file is one file to write.
type file struct {
	Name     string
	Contents []byte
}

// render produces the generated source for the model, gofmt'd.
func render(m *Model, pkg string) ([]file, error) {
	data, err := newTemplateData(m, pkg)
	if err != nil {
		return nil, err
	}

	files := []file{
		{Name: "variants_gen.go"},
		{Name: "tables_gen.go"},
		{Name: "emoji_gen.go"},
	}
	for i, f := range files {
		name := strings.TrimSuffix(f.Name, "_gen.go") + ".go.tmpl"

		var buf bytes.Buffer
		if err := tmpl.ExecuteTemplate(&buf, name, data); err != nil {
			return nil, fmt.Errorf("render %s: %w", f.Name, err)
		}
		src, err := format.Source(buf.Bytes())
		if err != nil {
			return nil, fmt.Errorf("gofmt %s: %w", f.Name, err)
		}
		files[i].Contents = src
	}
	return files, nil
}

// The types below are the shape the templates consume. Anything needing a Go
// literal is quoted here so the templates only ever interpolate.

type templateData struct {
	Package string
	Source  string
	Version string

	Variants      []variantData
	FirstSkinTone string
	LastSkinTone  string
	LastVariant   string

	EmojiBlob string // quoted literal holding every emoji
	NameBlob  string // quoted literal holding every name

	Bases  []baseEntry   // sorted by name, for binary search
	Styled []styledEntry // sorted by key, for binary search
	Groups []groupData
}

type variantData struct {
	Ident    string
	Token    string
	TokenLit string
}

// baseEntry is one row of the generated baseTable.
type baseEntry struct {
	Name    Span
	Plain   Span
	Comment string // the name, so the table stays readable
}

// styledEntry is one row of the generated variant tables.
type styledEntry struct {
	Key     uint32
	Emoji   Span
	Comment string
}

type groupData struct {
	Name  string
	Bases []baseData
}

type baseData struct {
	Ident string
	Doc   []string

	Index int // row in the base tables
}

// styledKey packs a row of baseTable and up to two variants into one integer,
// so the variant table is a sorted list of integers rather than a map.
func styledKey(base, a, b int) uint32 {
	return uint32(base)<<16 | uint32(a)<<8 | uint32(b)
}

func newTemplateData(m *Model, pkg string) (*templateData, error) {
	d := &templateData{
		Package:       pkg,
		Source:        m.Source,
		Version:       m.Version,
		FirstSkinTone: SkinTones[0].Ident,
		LastSkinTone:  SkinTones[len(SkinTones)-1].Ident,
		LastVariant:   m.Variants[len(m.Variants)-1].Ident,
	}

	variantValue := make(map[string]int, len(m.Variants))
	for i, v := range m.Variants {
		d.Variants = append(d.Variants, variantData{
			Ident: v.Ident, Token: v.Token, TokenLit: strconv.Quote(v.Token),
		})
		variantValue[v.Ident] = i + 1 // noVariant is 0
	}
	if n := len(m.Variants); n >= 1<<8 {
		return nil, fmt.Errorf("%d variants will not fit in a byte of the lookup key", n)
	}

	// Pack every emoji and every name into one string each.
	var sequences, names []string
	for _, b := range m.Bases {
		names = append(names, b.Name)
		sequences = append(sequences, b.Plain)
		for _, f := range b.Forms {
			sequences = append(sequences, f.Sequence)
		}
	}
	emojiBlob := NewBlob("emoji", sequences)
	nameBlob := NewBlob("names", names)
	d.EmojiBlob = quoteChunked(emojiBlob.Text)
	d.NameBlob = quoteChunked(nameBlob.Text)

	// baseTable is searched by name, so it is sorted by name.
	sorted := append([]*Base(nil), m.Bases...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Name < sorted[j].Name })
	if n := len(sorted); n >= 1<<16 {
		return nil, fmt.Errorf("%d emoji will not fit in the 16 bits the lookup key allows", n)
	}
	row := make(map[string]int, len(sorted))
	for i, b := range sorted {
		row[b.Name] = i
		d.Bases = append(d.Bases, baseEntry{
			Name:    nameBlob.Span(b.Name),
			Plain:   emojiBlob.Span(b.Plain),
			Comment: b.Name,
		})
	}

	for _, b := range m.Bases {
		for _, f := range b.Forms {
			a := variantValue[f.Variants[0]]
			second := 0
			if len(f.Variants) > 1 {
				second = variantValue[f.Variants[1]]
			}
			d.Styled = append(d.Styled, styledEntry{
				Key:     styledKey(row[b.Name], a, second),
				Emoji:   emojiBlob.Span(f.Sequence),
				Comment: f.Name,
			})
		}
	}
	sort.Slice(d.Styled, func(i, j int) bool { return d.Styled[i].Key < d.Styled[j].Key })

	for _, g := range m.Groups {
		gd := groupData{Name: g.Name}
		for _, b := range g.Bases {
			gd.Bases = append(gd.Bases, baseData{
				Ident: b.Ident,
				Doc:   doc(b),
				Index: row[b.Name],
			})
		}
		d.Groups = append(d.Groups, gd)
	}
	return d, nil
}

// doc writes the comment above a generated function.
func doc(b *Base) []string {
	lines := wrap(fmt.Sprintf("%s returns the %q emoji%s.", b.Ident, b.Name, sample(b)), 72)
	if len(b.Forms) == 0 {
		return lines
	}

	s := accepts(b)
	var sentence string
	switch {
	case s.pairsTones:
		// The two modifiers are one skin tone per figure, so their order is
		// what tells the figures apart.
		sentence = fmt.Sprintf("It accepts one skin tone for both figures, or one per figure in "+
			"the order they appear: %s.", strings.Join(s.idents, ", "))
	case s.arity > 1:
		sentence = fmt.Sprintf("It accepts a skin tone and a hair style, in either order, or "+
			"either on its own: %s.", strings.Join(s.idents, ", "))
	default:
		sentence = fmt.Sprintf("It accepts one variant: %s.", strings.Join(s.idents, ", "))
	}
	if b.Plain == "" {
		sentence += " Unicode defines this emoji only in modified forms, so calling it with no variants returns the empty string."
	} else {
		sentence += " Calling it with no variants returns the unmodified emoji, and an unsupported combination returns the empty string."
	}

	lines = append(lines, "")
	return append(lines, wrap(sentence, 72)...)
}

// shape describes how a base may be modified.
type shape struct {
	idents     []string // accepted variants, in declaration order
	arity      int      // most modifiers carried at once
	pairsTones bool     // a two-modifier form that is two skin tones
}

// sample shows the emoji itself in the doc comment, preferring the unmodified
// form and falling back to the first modified one.
func sample(b *Base) string {
	seq := b.Plain
	if seq == "" && len(b.Forms) > 0 {
		seq = b.Forms[0].Sequence
	}
	if seq == "" {
		return ""
	}
	return " " + seq
}

// accepts works out how a base may be modified, by inspecting its forms rather
// than assuming: an emoji taking two modifiers is either a single figure with a
// skin tone and a hair style, or two figures with a skin tone each, and the two
// cases read differently to a caller.
func accepts(b *Base) shape {
	var s shape
	seen := make(map[string]bool)
	for _, f := range b.Forms {
		if len(f.Variants) > s.arity {
			s.arity = len(f.Variants)
		}
		if len(f.Variants) == 2 && isSkinTone(f.Variants[1]) {
			s.pairsTones = true
		}
		for _, v := range f.Variants {
			seen[v] = true
		}
	}
	for _, v := range Variants() {
		if seen[v.Ident] {
			s.idents = append(s.idents, v.Ident)
		}
	}
	return s
}

func isSkinTone(ident string) bool {
	for _, v := range SkinTones {
		if v.Ident == ident {
			return true
		}
	}
	return false
}

// quote renders an emoji sequence as a Go string literal.
//
// strconv.Quote is not enough on its own: it leaves anything unicode.IsPrint
// accepts as a raw rune, which for emoji includes the variation selectors.
// Those are zero-width, so they would sit invisibly in the generated source
// and in diffs. Everything with no glyph of its own is escaped instead, which
// leaves the emoji, the skin tone modifiers and the regional indicators
// legible in the literal while the joiners and selectors stay explicit.
func quote(s string) string {
	var sb strings.Builder
	sb.WriteByte('"')
	for _, r := range s {
		sb.WriteString(escape(r))
	}
	sb.WriteByte('"')
	return sb.String()
}

// escape renders one rune as it should appear inside a Go string literal.
func escape(r rune) string {
	switch {
	case r == '"' || r == '\\':
		return `\` + string(r)
	case isVariationSelector(r) || !strconv.IsPrint(r):
		if r > 0xFFFF {
			return fmt.Sprintf(`\U%08X`, r)
		}
		return fmt.Sprintf(`\u%04X`, r)
	default:
		return string(r)
	}
}

// chunkWidth is roughly how many bytes of a blob go on one line of the
// generated source.
const chunkWidth = 96

// quoteChunked renders s as a Go string literal broken across many lines:
//
//	"" +
//		"first chunk" +
//		"second chunk"
//
// The blobs run to tens of thousands of characters. On one line that is a
// valid but hostile piece of source: editors that cap how long a line they
// will parse report phantom errors across the whole file, and every diff
// touching the blob rewrites one enormous line. Concatenated constants fold at
// compile time, so this costs nothing at run time.
//
// Lines are only ever broken between runes, never inside an escape sequence.
func quoteChunked(s string) string {
	var lines []string
	var cur strings.Builder
	for _, r := range s {
		piece := escape(r)
		if cur.Len() > 0 && cur.Len()+len(piece) > chunkWidth {
			lines = append(lines, cur.String())
			cur.Reset()
		}
		cur.WriteString(piece)
	}
	if cur.Len() > 0 {
		lines = append(lines, cur.String())
	}

	// The leading "" keeps every chunk on a line of its own, including the
	// first, and makes the empty blob render as a plain "".
	var sb strings.Builder
	sb.WriteString(`""`)
	for _, line := range lines {
		sb.WriteString(" +\n\t\"")
		sb.WriteString(line)
		sb.WriteByte('"')
	}
	return sb.String()
}

func isVariationSelector(r rune) bool { return r >= 0xFE00 && r <= 0xFE0F }

// wrap breaks a sentence into comment-width lines.
func wrap(s string, width int) []string {
	var lines []string
	line := ""
	for _, word := range strings.Fields(s) {
		switch {
		case line == "":
			line = word
		case len(line)+1+len(word) > width:
			lines = append(lines, line)
			line = word
		default:
			line += " " + word
		}
	}
	if line != "" {
		lines = append(lines, line)
	}
	return lines
}
