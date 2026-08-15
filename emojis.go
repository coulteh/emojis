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

// A span is a half-open range of one of the generated blobs. The tables hold
// spans rather than strings so that they stay arrays of plain integers, which
// the compiler writes into the binary; an array of strings would carry a
// pointer each and need relocating when the program loads.
type span struct{ off, end uint32 }

func (s span) in(blob string) string { return blob[s.off:s.end] }

// Lookup returns the emoji with the given Unicode name, styled with the given
// variants. The name is the CLDR short name with any modifiers stripped, which
// is what the generated functions are named after:
//
//	emojis.Lookup("thumbs up", emojis.DarkSkinTone) // 👍🏿
//	emojis.Lookup("family: man, boy")               // 👨‍👦
//
// An unknown name, or a combination of variants that Unicode does not define,
// returns the empty string. Prefer the generated functions where the emoji is
// known at compile time: they index the tables directly, while Lookup has to
// search them for the name.
func Lookup(name string, v ...Variant) string {
	row, ok := findBase(name)
	if !ok {
		return ""
	}
	return styled(row, v)
}

// styled returns the emoji at a row of the base tables, in the requested
// style. The generated functions call it with their own row, which they know
// at compile time and so never have to search for.
func styled(row int, v []Variant) string {
	switch len(v) {
	case 0:
		return baseEmoji[row].in(emojiBlob)
	case 1:
		return findStyled(row, v[0], noVariant)
	case 2:
		a, b := order(v[0], v[1])
		return findStyled(row, a, b)
	default:
		return ""
	}
}

// findBase binary searches baseNames, which the generator sorted by name.
//
// The search is written out rather than handed to sort.Search because the
// closure that takes would keep this off the inlining path, and these two
// searches are the whole cost of a lookup.
func findBase(name string) (int, bool) {
	lo, hi := 0, len(baseNames)
	for lo < hi {
		mid := int(uint(lo+hi) >> 1)
		if baseNames[mid].in(nameBlob) < name {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	if lo < len(baseNames) && baseNames[lo].in(nameBlob) == name {
		return lo, true
	}
	return 0, false
}

// findStyled binary searches styledKeys for one modified form.
func findStyled(row int, a, b Variant) string {
	// Out-of-range variants are rejected before they are packed into the key,
	// where they would otherwise corrupt the row number beside them.
	if a <= noVariant || a > lastVariant || b < noVariant || b > lastVariant {
		return ""
	}
	key := uint32(row)<<16 | uint32(a)<<8 | uint32(b)

	lo, hi := 0, len(styledKeys)
	for lo < hi {
		mid := int(uint(lo+hi) >> 1)
		if styledKeys[mid] < key {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	if lo < len(styledKeys) && styledKeys[lo] == key {
		return styledEmoji[lo].in(emojiBlob)
	}
	return ""
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
//
// The slice is built on demand rather than kept around, so a program that never
// calls this pays nothing for it.
func Names() []string {
	names := make([]string, len(baseNames))
	for i, n := range baseNames {
		names[i] = n.in(nameBlob) // baseNames is already sorted
	}
	return names
}
