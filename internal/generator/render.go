package generator

import (
	"bytes"
	"embed"
	"fmt"
	"go/format"
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
	data := newTemplateData(m, pkg)

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

	Plain  []entryData
	Forms  []formData
	Groups []groupData
}

type variantData struct {
	Ident    string
	Token    string
	TokenLit string
}

type entryData struct{ Name, Sequence string }

type formData struct {
	Name     string
	A, B     string
	Sequence string
}

type groupData struct {
	Name  string
	Bases []baseData
}

type baseData struct {
	Ident string
	Name  string
	Doc   []string
}

func newTemplateData(m *Model, pkg string) *templateData {
	d := &templateData{
		Package:       pkg,
		Source:        m.Source,
		Version:       m.Version,
		FirstSkinTone: SkinTones[0].Ident,
		LastSkinTone:  SkinTones[len(SkinTones)-1].Ident,
		LastVariant:   m.Variants[len(m.Variants)-1].Ident,
	}

	for _, v := range m.Variants {
		d.Variants = append(d.Variants, variantData{
			Ident: v.Ident, Token: v.Token, TokenLit: strconv.Quote(v.Token),
		})
	}

	for _, b := range m.Bases {
		d.Plain = append(d.Plain, entryData{
			Name:     strconv.Quote(b.Name),
			Sequence: quote(b.Plain),
		})
		for _, f := range b.Forms {
			fd := formData{
				Name:     strconv.Quote(b.Name),
				A:        f.Variants[0],
				B:        "noVariant",
				Sequence: quote(f.Sequence),
			}
			if len(f.Variants) > 1 {
				fd.B = f.Variants[1]
			}
			d.Forms = append(d.Forms, fd)
		}
	}

	for _, g := range m.Groups {
		gd := groupData{Name: g.Name}
		for _, b := range g.Bases {
			gd.Bases = append(gd.Bases, baseData{
				Ident: b.Ident,
				Name:  strconv.Quote(b.Name),
				Doc:   doc(b),
			})
		}
		d.Groups = append(d.Groups, gd)
	}
	return d
}

// doc writes the comment above a generated function.
func doc(b *Base) []string {
	lines := []string{
		fmt.Sprintf("%s returns the %q emoji%s.", b.Ident, b.Name, sample(b)),
	}
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
		switch {
		case r == '"' || r == '\\':
			sb.WriteByte('\\')
			sb.WriteRune(r)
		case isVariationSelector(r) || !strconv.IsPrint(r):
			if r > 0xFFFF {
				fmt.Fprintf(&sb, `\U%08X`, r)
			} else {
				fmt.Fprintf(&sb, `\u%04X`, r)
			}
		default:
			sb.WriteRune(r)
		}
	}
	sb.WriteByte('"')
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
