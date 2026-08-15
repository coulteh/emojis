package emojis

import (
	"strings"
	"testing"
)

func TestPlain(t *testing.T) {
	tests := []struct {
		got  string
		want string
	}{
		{GrinningFace(), "\U0001F600"},
		{ThumbsUp(), "\U0001F44D"},
		{RedHeart(), "❤️"},
		{Person(), "\U0001F9D1"},
		{FamilyManBoy(), "\U0001F468‍\U0001F466"},
		{KissWomanMan(), "\U0001F469‍❤️‍\U0001F48B‍\U0001F468"},
		{Keycap1(), "1️⃣"},
		{KeycapHash(), "#️⃣"},
		{FirstPlaceMedal(), "\U0001F947"},
		{FlagUnitedStates(), "\U0001F1FA\U0001F1F8"},
	}
	for _, tt := range tests {
		if tt.got != tt.want {
			t.Errorf("got %q, want %q", tt.got, tt.want)
		}
	}
}

func TestSkinTone(t *testing.T) {
	tests := []struct {
		got  string
		want string
	}{
		{ThumbsUp(LightSkinTone), "\U0001F44D\U0001F3FB"},
		{ThumbsUp(DarkSkinTone), "\U0001F44D\U0001F3FF"},
		{ThumbsUp(MediumLightSkinTone), "\U0001F44D\U0001F3FC"},
		{WavingHand(MediumDarkSkinTone), "\U0001F44B\U0001F3FE"},
	}
	for _, tt := range tests {
		if tt.got != tt.want {
			t.Errorf("got %q, want %q", tt.got, tt.want)
		}
	}
}

// A skin tone and a hair style name one figure, so either order means the same
// thing and both must resolve.
func TestToneAndHairCommute(t *testing.T) {
	want := "\U0001F9D1\U0001F3FD‍\U0001F9B0" // 🧑🏽‍🦰
	if got := Person(MediumSkinTone, RedHair); got != want {
		t.Errorf("Person(MediumSkinTone, RedHair) = %q, want %q", got, want)
	}
	if got := Person(RedHair, MediumSkinTone); got != want {
		t.Errorf("Person(RedHair, MediumSkinTone) = %q, want %q", got, want)
	}
}

// Two skin tones name one figure each, so the order is the meaning and must be
// preserved rather than normalised away.
func TestTwoTonesKeepOrder(t *testing.T) {
	lightDark := MenHoldingHands(LightSkinTone, DarkSkinTone)
	darkLight := MenHoldingHands(DarkSkinTone, LightSkinTone)

	if lightDark == darkLight {
		t.Fatalf("both orderings returned %q; the order distinguishes the figures", lightDark)
	}
	if want := "\U0001F468\U0001F3FB‍\U0001F91D‍\U0001F468\U0001F3FF"; lightDark != want {
		t.Errorf("MenHoldingHands(Light, Dark) = %q, want %q", lightDark, want)
	}
	if want := "\U0001F468\U0001F3FF‍\U0001F91D‍\U0001F468\U0001F3FB"; darkLight != want {
		t.Errorf("MenHoldingHands(Dark, Light) = %q, want %q", darkLight, want)
	}
	// A single tone applies to both figures.
	if want := "\U0001F46C\U0001F3FB"; MenHoldingHands(LightSkinTone) != want {
		t.Errorf("MenHoldingHands(Light) = %q, want %q", MenHoldingHands(LightSkinTone), want)
	}
}

// Passing a variant to an emoji that has none does not compile, so the only
// combinations reachable at run time are the wrong ones for an emoji that does
// take variants.
func TestUnsupportedCombinations(t *testing.T) {
	tests := []struct {
		name string
		got  string
	}{
		{"hair style on a hand", ThumbsUp(RedHair)},
		{"two variants where one is allowed", ThumbsUp(LightSkinTone, DarkSkinTone)},
		{"more variants than any emoji takes", Person(LightSkinTone, RedHair, DarkSkinTone)},
		{"zero value variant", ThumbsUp(Variant(0))},
		{"out of range variant", ThumbsUp(Variant(99))},
	}
	for _, tt := range tests {
		if tt.got != "" {
			t.Errorf("%s: got %q, want empty string", tt.name, tt.got)
		}
	}
}

// Unicode defines these only in modified forms; there is no plain emoji to
// return.
func TestModifiedFormsOnly(t *testing.T) {
	if got := KissPersonPerson(); got != "" {
		t.Errorf("KissPersonPerson() = %q, want empty string", got)
	}
	if got := KissPersonPerson(LightSkinTone, MediumLightSkinTone); got == "" {
		t.Error("KissPersonPerson(Light, MediumLight) returned empty string")
	}
}

func TestLookup(t *testing.T) {
	if got, want := Lookup("thumbs up"), ThumbsUp(); got != want {
		t.Errorf("Lookup(%q) = %q, want %q", "thumbs up", got, want)
	}
	if got, want := Lookup("thumbs up", DarkSkinTone), ThumbsUp(DarkSkinTone); got != want {
		t.Errorf("Lookup with variant = %q, want %q", got, want)
	}
	if got, want := Lookup("family: man, boy"), FamilyManBoy(); got != want {
		t.Errorf("Lookup(%q) = %q, want %q", "family: man, boy", got, want)
	}
	if got := Lookup("no such emoji"); got != "" {
		t.Errorf("Lookup of an unknown name = %q, want empty string", got)
	}
}

func TestNames(t *testing.T) {
	names := Names()
	if len(names) != len(baseNames) {
		t.Errorf("Names() returned %d names, want %d", len(names), len(baseNames))
	}
	for i := 1; i < len(names); i++ {
		if names[i-1] >= names[i] {
			t.Fatalf("Names() is not sorted: %q before %q", names[i-1], names[i])
		}
	}
	// Every name resolves, apart from the few defined only with modifiers.
	var modifiedOnly int
	for _, n := range names {
		if Lookup(n) == "" {
			modifiedOnly++
		}
	}
	if modifiedOnly > 2 {
		t.Errorf("%d names have no unmodified form, expected at most 2", modifiedOnly)
	}
}

func TestVariantString(t *testing.T) {
	if got, want := DarkSkinTone.String(), "dark skin tone"; got != want {
		t.Errorf("DarkSkinTone.String() = %q, want %q", got, want)
	}
	if got, want := Variant(0).String(), "unknown variant"; got != want {
		t.Errorf("Variant(0).String() = %q, want %q", got, want)
	}
	if got, want := Variant(99).String(), "unknown variant"; got != want {
		t.Errorf("Variant(99).String() = %q, want %q", got, want)
	}
}

func TestParseVariant(t *testing.T) {
	if v, ok := ParseVariant("dark skin tone"); !ok || v != DarkSkinTone {
		t.Errorf("ParseVariant(%q) = %v, %v; want DarkSkinTone, true", "dark skin tone", v, ok)
	}
	if _, ok := ParseVariant(""); ok {
		t.Error("ParseVariant(\"\") reported a match")
	}
	if _, ok := ParseVariant("chartreuse skin tone"); ok {
		t.Error("ParseVariant of an unknown token reported a match")
	}
	// Round-trips with String for every declared variant.
	for v := firstSkinTone; v <= lastVariant; v++ {
		got, ok := ParseVariant(v.String())
		if !ok || got != v {
			t.Errorf("ParseVariant(%q) = %v, %v; want %v, true", v.String(), got, ok, v)
		}
	}
}

// The tables and the functions must agree: every modified form has to come back
// through Lookup when asked for by name and variants.
func TestEveryFormIsReachable(t *testing.T) {
	for i, key := range styledKeys {
		row := int(key >> 16)
		a, b := Variant(key>>8&0xFF), Variant(key&0xFF)
		name := baseNames[row].in(nameBlob)
		want := styledEmoji[i].in(emojiBlob)

		got := Lookup(name, a)
		if b != noVariant {
			got = Lookup(name, a, b)
		}
		if got != want {
			t.Errorf("Lookup(%q, %v, %v) = %q, want %q", name, a, b, got, want)
		}
	}
}

// Every name must be findable, which is what the binary search depends on.
func TestBaseNamesAreSorted(t *testing.T) {
	for i := 1; i < len(baseNames); i++ {
		prev, cur := baseNames[i-1].in(nameBlob), baseNames[i].in(nameBlob)
		if prev >= cur {
			t.Fatalf("baseNames is not sorted: %q before %q", prev, cur)
		}
	}
	for i, n := range baseNames {
		if row, ok := findBase(n.in(nameBlob)); !ok || row != i {
			t.Errorf("findBase(%q) = %d, %v; want %d, true", n.in(nameBlob), row, ok, i)
		}
	}
}

// The binary search over styledKeys depends on their order too.
func TestStyledKeysAreSorted(t *testing.T) {
	if len(styledKeys) != len(styledEmoji) {
		t.Fatalf("styledKeys has %d entries but styledEmoji has %d", len(styledKeys), len(styledEmoji))
	}
	for i := 1; i < len(styledKeys); i++ {
		if styledKeys[i-1] >= styledKeys[i] {
			t.Fatalf("styledKeys is not sorted at %d: %d before %d", i, styledKeys[i-1], styledKeys[i])
		}
	}
}

// No generated emoji should contain a stray separator or be empty by accident.
func TestGeneratedDataIsSane(t *testing.T) {
	var empty int
	for i, n := range baseNames {
		name := n.in(nameBlob)
		if name == "" {
			t.Error("baseNames contains an empty name")
		}
		seq := baseEmoji[i].in(emojiBlob)
		if seq == "" {
			empty++
			continue
		}
		if strings.ContainsAny(seq, " \t\n") {
			t.Errorf("emoji %q contains whitespace: %q", name, seq)
		}
	}
	if empty > 2 {
		t.Errorf("%d emoji have no unmodified form, expected at most 2", empty)
	}
	if len(baseNames) < 1500 {
		t.Errorf("only %d emoji generated; the data looks truncated", len(baseNames))
	}
	if len(styledKeys) < 1500 {
		t.Errorf("only %d modified forms generated; the data looks truncated", len(styledKeys))
	}
}
