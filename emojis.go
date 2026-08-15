// Package emojis returns Unicode emoji by name.
//
// Every emoji has a function named after its CLDR short name:
//
//	emojis.ThumbsUp()    // 👍
//	emojis.RedHeart()    // ❤️
//	emojis.FamilyManBoy() // 👨‍👦
//
// Emoji that can be restyled — with a skin tone, a hair style, or both — take
// those as arguments rather than having a function each:
//
//	emojis.ThumbsUp(emojis.DarkSkinTone)               // 👍🏿
//	emojis.Person(emojis.MediumSkinTone, emojis.RedHair) // 🧑🏽‍🦰
//	emojis.MenHoldingHands(emojis.LightSkinTone, emojis.DarkSkinTone) // 👨🏻‍🤝‍👨🏿
//
// The data comes from Unicode's emoji-test.txt and is regenerated with
// "go generate ./...".
package emojis

import "sort"

//go:generate go run ./internal

// A Variant is a styling an emoji can be asked for: one of the five skin
// tones, or one of the hair styles.
type Variant int

// String returns the variant's Unicode name, such as "dark skin tone".
func (v Variant) String() string {
	if v <= noVariant || v > lastVariant {
		return "unknown variant"
	}
	return variantTokens[v]
}

// isSkinTone reports whether v is one of the five skin tones.
func (v Variant) isSkinTone() bool { return v >= firstSkinTone && v <= lastSkinTone }

// ParseVariant returns the Variant with the given Unicode name, as produced by
// Variant.String.
func ParseVariant(name string) (Variant, bool) {
	for v, token := range variantTokens {
		if token == name && Variant(v) != noVariant {
			return Variant(v), true
		}
	}
	return noVariant, false
}

// formKey identifies one modified form of an emoji. Both slots are always set;
// b is noVariant for emoji carrying a single modifier.
type formKey struct {
	name string
	a, b Variant
}

// Lookup returns the emoji with the given Unicode name, styled with the given
// variants. The name is the CLDR short name with any modifiers stripped, which
// is what the generated functions pass:
//
//	emojis.Lookup("thumbs up", emojis.DarkSkinTone) // 👍🏿
//	emojis.Lookup("family: man, boy")               // 👨‍👦
//
// An unknown name, or a combination of variants that Unicode does not define,
// returns the empty string. Prefer the generated functions where the emoji is
// known at compile time; Lookup is for choosing one at runtime.
func Lookup(name string, v ...Variant) string {
	switch len(v) {
	case 0:
		return plainForms[name]
	case 1:
		return variantForms[formKey{name, v[0], noVariant}]
	case 2:
		a, b := order(v[0], v[1])
		return variantForms[formKey{name, a, b}]
	default:
		return ""
	}
}

// order puts a skin tone ahead of a hair style, which is the order Unicode
// names them in, so that Person(RedHair, MediumSkinTone) resolves just as
// Person(MediumSkinTone, RedHair) does.
//
// Two skin tones keep the caller's order, because there the order is the
// meaning: it says which tone belongs to which figure in emoji such as
// MenHoldingHands.
func order(a, b Variant) (Variant, Variant) {
	if b.isSkinTone() && !a.isSkinTone() {
		return b, a
	}
	return a, b
}

// Names returns the Unicode name of every emoji in the package, sorted. Each
// is a name Lookup accepts.
func Names() []string {
	names := make([]string, 0, len(plainForms))
	for name := range plainForms {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
