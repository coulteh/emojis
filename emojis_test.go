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

func TestUnsupportedCombinations(t *testing.T) {
	tests := []struct {
		name string
		got  string
	}{
		{"variant on an emoji that takes none", GrinningFace(DarkSkinTone)},
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
	if len(names) != len(plainForms) {
		t.Errorf("Names() returned %d names, want %d", len(names), len(plainForms))
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

// The tables and the functions must agree: every form in variantForms has to be
// reachable through Lookup with the variants it was keyed by.
func TestEveryFormIsReachable(t *testing.T) {
	for key, want := range variantForms {
		var got string
		if key.b == noVariant {
			got = Lookup(key.name, key.a)
		} else {
			got = Lookup(key.name, key.a, key.b)
		}
		if got != want {
			t.Errorf("Lookup(%q, %v, %v) = %q, want %q", key.name, key.a, key.b, got, want)
		}
	}
}

// No generated emoji should contain a stray separator or be empty by accident.
func TestGeneratedDataIsSane(t *testing.T) {
	var empty int
	for name, seq := range plainForms {
		if name == "" {
			t.Error("plainForms contains an empty name")
		}
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
	if len(plainForms) < 1500 {
		t.Errorf("only %d emoji generated; the data looks truncated", len(plainForms))
	}
	if len(variantForms) < 1500 {
		t.Errorf("only %d modified forms generated; the data looks truncated", len(variantForms))
	}
}
