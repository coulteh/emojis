package generator

import (
	"fmt"
	"log/slog"
	"sort"
	"strings"
)

// maxVariants is how many modifiers a single emoji may carry. The generated
// lookup key holds exactly this many slots, so Build fails rather than silently
// dropping emoji if a future Unicode release exceeds it.
const maxVariants = 2

// A Form is one concrete emoji: a base plus a specific set of modifiers.
type Form struct {
	Variants []string // generated Variant identifiers, in CLDR order
	Sequence string
	Name     string // full CLDR name
}

// A Base becomes one generated function.
type Base struct {
	Ident    string // Go identifier, e.g. "ThumbsUp"
	Name     string // CLDR name without modifiers, e.g. "thumbs up", "kiss: woman, man"
	Group    string
	Subgroup string
	Plain    string // sequence with no modifiers; empty if the base has no unmodified form
	Forms    []Form // modified forms only; empty if the emoji takes no variants
}

// A Group is a run of bases sharing a Unicode group, kept so the generated file
// mirrors the ordering and section headers of the source data.
type Group struct {
	Name  string
	Bases []*Base
}

// A Model is a parsed dataset reshaped into the package to generate.
type Model struct {
	Source   string
	Version  string
	Groups   []*Group
	Bases    []*Base // flattened, same order as Groups
	Variants []VariantDef
}

// Build groups the dataset's emoji into one Base per generated function.
func Build(ds *Dataset) (*Model, error) {
	m := &Model{Source: ds.Source, Version: ds.Version, Variants: Variants()}

	byName := make(map[string]*Base, len(ds.Emojis))
	groups := make(map[string]*Group)
	idents := newIdentSet()
	unknown := make(map[string]map[string]bool)

	for _, e := range ds.Emojis {
		name, variants, rest, err := split(e.Name)
		if err != nil {
			return nil, err
		}
		for _, tok := range rest {
			if namesAFigure(tok) {
				continue
			}
			if unknown[tok] == nil {
				unknown[tok] = make(map[string]bool)
			}
			unknown[tok][name] = true
		}

		base, ok := byName[name]
		if !ok {
			base = &Base{
				Ident:    idents.add(name),
				Name:     name,
				Group:    e.Group,
				Subgroup: e.Subgroup,
			}
			byName[name] = base
			m.Bases = append(m.Bases, base)

			g, ok := groups[e.Group]
			if !ok {
				g = &Group{Name: e.Group}
				groups[e.Group] = g
				m.Groups = append(m.Groups, g)
			}
			g.Bases = append(g.Bases, base)
		}

		if len(variants) == 0 {
			if base.Plain != "" {
				return nil, fmt.Errorf("emoji %q has two unmodified forms: %q and %q", name, base.Plain, e.Sequence)
			}
			base.Plain = e.Sequence
			continue
		}
		if dup := find(base.Forms, variants); dup != nil {
			return nil, fmt.Errorf("emoji %q has two forms for %v: %q and %q", name, variants, dup.Sequence, e.Sequence)
		}
		base.Forms = append(base.Forms, Form{Variants: variants, Sequence: e.Sequence, Name: e.Name})
	}

	withVariants, forms := 0, 0
	for _, b := range m.Bases {
		if len(b.Forms) > 0 {
			withVariants++
			forms += len(b.Forms)
		}
	}
	slog.Info("built functions",
		"functions", len(m.Bases), "groups", len(m.Groups),
		"take_variants", withVariants, "modified_forms", forms)
	reportUnrecognisedTokens(unknown)
	return m, nil
}

// sharedTokenBases is how many emoji have to share an unrecognised token
// before it is reported as a styling rather than a one-off.
//
// A flag or a keycap qualifier belongs to a single emoji: every flag is named
// "flag: somewhere", so the country appears under one name. A styling is the
// opposite, since it applies across many emoji: the six hair styles Unicode
// added in emoji 12.0 each turned up under man, woman and person at once. Two
// is comfortably between the two: across every published release, no
// unrecognised token has ever appeared under more than one name.
const sharedTokenBases = 2

// reportUnrecognisedTokens warns about a qualifier that looks like a styling
// the generator does not know about.
//
// A token it has no modifier for silently becomes part of a function name, so
// a release that invents one would quietly turn what should be a Variant into
// hundreds of separate functions. That is what happened in emoji 12.0, where
// "blond hair" arrived alongside the rest of the hair styles.
func reportUnrecognisedTokens(unknown map[string]map[string]bool) {
	var shared []string
	for tok, names := range unknown {
		if len(names) >= sharedTokenBases {
			shared = append(shared, tok)
		}
	}
	if len(shared) == 0 {
		return
	}
	sort.Strings(shared)
	for _, tok := range shared {
		slog.Warn("unrecognised qualifier shared by several emoji; if it is a styling, add it to SkinTones or HairStyles, otherwise it becomes part of the function name",
			"token", tok, "emoji", len(unknown[tok]))
	}
}

// split separates a CLDR name into the name of the function that will return it
// and the modifiers that select this particular form.
//
//	"thumbs up"                            -> "thumbs up",      []
//	"thumbs up: dark skin tone"            -> "thumbs up",      [DarkSkinTone]
//	"person: medium skin tone, red hair"   -> "person",         [MediumSkinTone, RedHair]
//	"kiss: woman, man, light skin tone"    -> "kiss: woman, man", [LightSkinTone]
//	"family: man, boy"                     -> "family: man, boy", []
//
// The third return is the tokens that were not modifiers and so ended up in
// the name, which Build inspects for stylings Unicode has newly invented.
func split(cldr string) (string, []string, []string, error) {
	head, tail, ok := strings.Cut(cldr, ":")
	if !ok {
		return cldr, nil, nil, nil
	}
	if strings.Contains(tail, ":") {
		return "", nil, nil, fmt.Errorf("emoji name %q has more than one %q separator", cldr, ":")
	}

	var variants, rest []string
	for _, tok := range strings.Split(tail, ",") {
		tok = strings.TrimSpace(tok)
		if ident, isVariant := variantIdent(tok); isVariant {
			variants = append(variants, ident)
			continue
		}
		rest = append(rest, tok)
	}
	if len(variants) > maxVariants {
		return "", nil, nil, fmt.Errorf("emoji name %q carries %d modifiers, more than the supported %d",
			cldr, len(variants), maxVariants)
	}

	name := strings.TrimSpace(head)
	if len(rest) > 0 {
		name += ": " + strings.Join(rest, ", ")
	}
	return name, variants, rest, nil
}

func find(forms []Form, variants []string) *Form {
	for i, f := range forms {
		if len(f.Variants) != len(variants) {
			continue
		}
		same := true
		for j := range variants {
			if f.Variants[j] != variants[j] {
				same = false
				break
			}
		}
		if same {
			return &forms[i]
		}
	}
	return nil
}
