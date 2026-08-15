package generator

import (
	"io"
	"log/slog"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

func TestMain(m *testing.M) {
	// The generator logs as it works; the tests do not need to see it.
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	m.Run()
}

func TestIdent(t *testing.T) {
	tests := []struct{ name, want string }{
		{"grinning face", "GrinningFace"},
		{"thumbs up", "ThumbsUp"},
		// Existing capitalisation survives.
		{"ID button", "IDButton"},
		{"ATM sign", "ATMSign"},
		{"SOS button", "SOSButton"},
		{"A button (blood type)", "AButtonBloodType"},
		// Apostrophes elide rather than splitting a word.
		{"man’s shoe", "MansShoe"},
		{"eight o’clock", "EightOclock"},
		{"flag: Côte d’Ivoire", "FlagCoteDIvoire"},
		// Accented letters fold to ASCII.
		{"flag: Åland Islands", "FlagAlandIslands"},
		{"flag: São Tomé & Príncipe", "FlagSaoTomeAndPrincipe"},
		{"flag: Curaçao", "FlagCuracao"},
		{"flag: Türkiye", "FlagTurkiye"},
		// Quotation marks are separators, not letters.
		{"Japanese “secret” button", "JapaneseSecretButton"},
		// Symbols that carry meaning are spelled out.
		{"keycap: #", "KeycapHash"},
		{"keycap: *", "KeycapAsterisk"},
		{"keycap: 10", "Keycap10"},
		{"flag: Antigua & Barbuda", "FlagAntiguaAndBarbuda"},
		// An identifier may not start with a digit.
		{"1st place medal", "FirstPlaceMedal"},
		{"2nd place medal", "SecondPlaceMedal"},
		{"3rd place medal", "ThirdPlaceMedal"},
		{"100", "OneHundredThousand"}, // not a real name; exercises the fallback
		// Hyphens split words.
		{"t-rex", "TRex"},
		{"family: man, boy", "FamilyManBoy"},
	}
	for _, tt := range tests {
		// The fallback case above is only checked for validity, not exact text.
		if tt.name == "100" {
			if got := ident(tt.name); got == "" || got[0] < 'A' || got[0] > 'Z' {
				t.Errorf("ident(%q) = %q, want an identifier starting with a capital", tt.name, got)
			}
			continue
		}
		if got := ident(tt.name); got != tt.want {
			t.Errorf("ident(%q) = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestIdentSetDisambiguates(t *testing.T) {
	s := newIdentSet()
	// Two names that reduce to the same identifier.
	if got := s.add("red heart"); got != "RedHeart" {
		t.Fatalf("first add = %q, want RedHeart", got)
	}
	if got := s.add("red (heart)"); got != "RedHeart2" {
		t.Errorf("colliding add = %q, want RedHeart2", got)
	}
	if got := s.add("red -heart-"); got != "RedHeart3" {
		t.Errorf("second colliding add = %q, want RedHeart3", got)
	}
}

func TestSplit(t *testing.T) {
	tests := []struct {
		cldr     string
		name     string
		variants []string
	}{
		{"thumbs up", "thumbs up", nil},
		{"thumbs up: dark skin tone", "thumbs up", []string{"DarkSkinTone"}},
		{"person: medium skin tone, red hair", "person", []string{"MediumSkinTone", "RedHair"}},
		{"person: bald", "person", []string{"Bald"}},
		// Non-modifier tokens stay in the name.
		{"family: man, boy", "family: man, boy", nil},
		{"family: man, woman, girl, boy", "family: man, woman, girl, boy", nil},
		{"kiss: woman, man", "kiss: woman, man", nil},
		{"flag: United States", "flag: United States", nil},
		{"keycap: #", "keycap: #", nil},
		// Mixed: person tokens name the emoji, tones modify it.
		{"kiss: woman, man, light skin tone, dark skin tone", "kiss: woman, man",
			[]string{"LightSkinTone", "DarkSkinTone"}},
	}
	for _, tt := range tests {
		name, variants, err := split(tt.cldr)
		if err != nil {
			t.Errorf("split(%q): %v", tt.cldr, err)
			continue
		}
		if name != tt.name {
			t.Errorf("split(%q) name = %q, want %q", tt.cldr, name, tt.name)
		}
		if strings.Join(variants, ",") != strings.Join(tt.variants, ",") {
			t.Errorf("split(%q) variants = %v, want %v", tt.cldr, variants, tt.variants)
		}
	}
}

func TestSplitRejectsTooManyModifiers(t *testing.T) {
	_, _, err := split("crowd: light skin tone, dark skin tone, medium skin tone")
	if err == nil {
		t.Fatal("split accepted more modifiers than the lookup key can hold")
	}
}

const sampleData = `# emoji-test.txt
# Version: 17.0

# group: Smileys & Emotion

# subgroup: face-smiling
1F600                     ; fully-qualified     # 😀 E1.0 grinning face

# group: People & Body

# subgroup: hand-fingers-closed
1F44D                     ; fully-qualified     # 👍 E0.6 thumbs up
1F44D 1F3FB               ; fully-qualified     # 👍🏻 E1.0 thumbs up: light skin tone
1F44D 1F3FF               ; fully-qualified     # 👍🏿 E1.0 thumbs up: dark skin tone

# subgroup: person
1F9D1                     ; fully-qualified     # 🧑 E5.0 person
1F9D1 1F3FD               ; fully-qualified     # 🧑🏽 E5.0 person: medium skin tone
1F9D1 1F3FD 200D 1F9B0    ; fully-qualified     # 🧑🏽‍🦰 E12.1 person: medium skin tone, red hair
1F9D1 200D 1F91D 200D 1F9D1 ; minimally-qualified # 🧑‍🤝‍🧑 E12.0 people holding hands
1F3FB                     ; component           # 🏻 E1.0 light skin tone

# subgroup: family
1F468 200D 1F466          ; fully-qualified     # 👨‍👦 E4.0 family: man, boy
`

func parseSample(t *testing.T) *Dataset {
	t.Helper()
	ds, err := Parse(strings.NewReader(sampleData), "sample")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return ds
}

func TestParse(t *testing.T) {
	ds := parseSample(t)

	if ds.Version != "17.0" {
		t.Errorf("version = %q, want %q", ds.Version, "17.0")
	}
	// The minimally-qualified and component rows are skipped.
	if len(ds.Emojis) != 8 {
		t.Fatalf("parsed %d emoji, want 8", len(ds.Emojis))
	}

	first := ds.Emojis[0]
	if first.Name != "grinning face" || first.Sequence != "\U0001F600" {
		t.Errorf("first emoji = %+v", first)
	}
	if first.Group != "Smileys & Emotion" || first.Subgroup != "face-smiling" {
		t.Errorf("first emoji group = %q/%q", first.Group, first.Subgroup)
	}
	if last := ds.Emojis[len(ds.Emojis)-1]; last.Group != "People & Body" || last.Subgroup != "family" {
		t.Errorf("last emoji group = %q/%q", last.Group, last.Subgroup)
	}
}

func TestParseRejectsBadData(t *testing.T) {
	tests := []struct{ name, data string }{
		{"no version header", "1F600 ; fully-qualified # 😀 E1.0 grinning face\n"},
		{"no emoji at all", "# Version: 17.0\n# group: X\n"},
		{"missing status separator", "# Version: 17.0\n# group: X\n1F600 fully-qualified # 😀 E1.0 grinning face\n"},
		{"unparseable comment", "# Version: 17.0\n# group: X\n1F600 ; fully-qualified # grinning face\n"},
		{"bad code point", "# Version: 17.0\n# group: X\nZZZZ ; fully-qualified # 😀 E1.0 grinning face\n"},
		// The comment repeats the sequence, so a mismatch means a misread row.
		{"comment disagrees with code points", "# Version: 17.0\n# group: X\n1F600 ; fully-qualified # 😁 E1.0 grinning face\n"},
		{"emoji before any group", "# Version: 17.0\n1F600 ; fully-qualified # 😀 E1.0 grinning face\n"},
	}
	for _, tt := range tests {
		if _, err := Parse(strings.NewReader(tt.data), "sample"); err == nil {
			t.Errorf("%s: Parse succeeded, want an error", tt.name)
		}
	}
}

func TestBuild(t *testing.T) {
	m, err := Build(parseSample(t))
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	if len(m.Bases) != 4 {
		t.Fatalf("built %d bases, want 4", len(m.Bases))
	}
	if len(m.Groups) != 2 {
		t.Errorf("built %d groups, want 2", len(m.Groups))
	}

	byIdent := map[string]*Base{}
	for _, b := range m.Bases {
		byIdent[b.Ident] = b
	}

	thumbs, ok := byIdent["ThumbsUp"]
	if !ok {
		t.Fatal("no ThumbsUp base")
	}
	if thumbs.Plain != "\U0001F44D" {
		t.Errorf("ThumbsUp plain = %q", thumbs.Plain)
	}
	if len(thumbs.Forms) != 2 {
		t.Errorf("ThumbsUp has %d forms, want 2", len(thumbs.Forms))
	}

	person, ok := byIdent["Person"]
	if !ok {
		t.Fatal("no Person base")
	}
	if got := accepts(person); got.arity != 2 || got.pairsTones {
		t.Errorf("Person shape = %+v, want arity 2 and a hair style rather than a tone pair", got)
	}

	// Group ordering follows the source file.
	if m.Groups[0].Name != "Smileys & Emotion" || m.Groups[1].Name != "People & Body" {
		t.Errorf("groups = %q, %q", m.Groups[0].Name, m.Groups[1].Name)
	}
}

func TestBuildRejectsDuplicates(t *testing.T) {
	dup := "# Version: 17.0\n# group: X\n" +
		"1F600 ; fully-qualified # 😀 E1.0 grinning face\n" +
		"1F601 ; fully-qualified # 😁 E1.0 grinning face\n"
	ds, err := Parse(strings.NewReader(dup), "sample")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if _, err := Build(ds); err == nil {
		t.Error("Build accepted two unmodified forms for one name")
	}
}

func TestRenderProducesValidGo(t *testing.T) {
	m, err := Build(parseSample(t))
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	// render gofmts each file, so it fails if the templates emit invalid Go.
	files, err := render(m, "emojis")
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if len(files) != 3 {
		t.Fatalf("rendered %d files, want 3", len(files))
	}
	for _, f := range files {
		src := string(f.Contents)
		if !strings.HasPrefix(src, "// Code generated by") {
			t.Errorf("%s is missing the generated-code header", f.Name)
		}
		if !strings.Contains(src, "package emojis") {
			t.Errorf("%s is missing the package clause", f.Name)
		}
	}
	// Only emoji that can be restyled take an argument, so passing a variant to
	// one that cannot is a compile error rather than an empty string.
	emoji := string(files[2].Contents)
	for _, want := range []*regexp.Regexp{
		regexp.MustCompile(`func ThumbsUp\(v \.\.\.Variant\) string \{ return styled\(\d+, v\) \}`),
		regexp.MustCompile(`func Person\(v \.\.\.Variant\) string \{ return styled\(\d+, v\) \}`),
		regexp.MustCompile(`func GrinningFace\(\) string \{ return emojiBlob\[\d+:\d+\] \}`),
		regexp.MustCompile(`func FamilyManBoy\(\) string \{ return emojiBlob\[\d+:\d+\] \}`),
	} {
		if !want.MatchString(emoji) {
			t.Errorf("emoji_gen.go has nothing matching:\n\t%s", want)
		}
	}

	// The offsets the functions slice with must actually name their emoji in
	// the blob the tables declare.
	blob := blobConst(t, string(files[1].Contents), "emojiBlob")
	for _, tt := range []struct{ fn, want string }{
		{"GrinningFace", "\U0001F600"},
		{"FamilyManBoy", "\U0001F468‍\U0001F466"},
	} {
		re := regexp.MustCompile(tt.fn + `\(\) string \{ return emojiBlob\[(\d+):(\d+)\] \}`)
		m := re.FindStringSubmatch(emoji)
		if m == nil {
			t.Errorf("no generated %s slicing the blob", tt.fn)
			continue
		}
		off, _ := strconv.Atoi(m[1])
		end, _ := strconv.Atoi(m[2])
		if got := blob[off:end]; got != tt.want {
			t.Errorf("%s slices emojiBlob[%d:%d] = %q, want %q", tt.fn, off, end, got, tt.want)
		}
	}
}

// blobConst pulls one of the generated string constants back out of the
// source. The blobs are emitted as many concatenated literals across as many
// lines, so this joins them back up the way the compiler would.
func blobConst(t *testing.T, src, name string) string {
	t.Helper()
	block := regexp.MustCompile(`(?ms)^const ` + name + ` = (.*?)\n\n`).FindStringSubmatch(src)
	if block == nil {
		t.Fatalf("no %s constant in the generated tables", name)
	}
	literals := regexp.MustCompile(`"(?:[^"\\]|\\.)*"`).FindAllString(block[1], -1)
	if len(literals) < 2 {
		t.Errorf("%s is in %d literal(s); it should be split across lines", name, len(literals))
	}

	var sb strings.Builder
	for _, lit := range literals {
		s, err := strconv.Unquote(lit)
		if err != nil {
			t.Fatalf("%s contains an invalid Go string literal %s: %v", name, lit, err)
		}
		sb.WriteString(s)
	}
	return sb.String()
}

// No generated line may be long enough to upset an editor.
func TestGeneratedLinesAreReasonable(t *testing.T) {
	m, err := Build(parseSample(t))
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	files, err := render(m, "emojis")
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	for _, f := range files {
		for i, line := range strings.Split(string(f.Contents), "\n") {
			if len(line) > 200 {
				t.Errorf("%s:%d is %d bytes long", f.Name, i+1, len(line))
			}
		}
	}
}

func TestBlobPacksContainedStrings(t *testing.T) {
	// "\U0001F44D" is inside "\U0001F44D\U0001F3FB" and "b" is inside "abc",
	// so neither costs any bytes of its own.
	items := []string{"\U0001F44D", "\U0001F44D\U0001F3FB", "abc", "b", "", "abc"}
	b := NewBlob("test", items)

	for _, s := range items {
		span := b.Span(s)
		if got := b.Text[span.Off:span.End]; got != s {
			t.Errorf("Span(%q) points at %q", s, got)
		}
	}
	if want := len("\U0001F44D\U0001F3FB") + len("abc"); len(b.Text) != want {
		t.Errorf("blob is %d bytes (%q), want %d with nothing stored twice", len(b.Text), b.Text, want)
	}
	if got := b.Span(""); got != (Span{}) {
		t.Errorf(`Span("") = %+v, want the zero span`, got)
	}
	if got := b.Span("not packed"); got != (Span{}) {
		t.Errorf("Span of an unpacked string = %+v, want the zero span", got)
	}
}

// The blob must be identical from one run to the next, or every regeneration
// would churn the whole generated package.
func TestBlobIsDeterministic(t *testing.T) {
	first := NewBlob("test", []string{"cherry", "err", "banana", "an", "apple", "ple"})
	second := NewBlob("test", []string{"ple", "an", "apple", "banana", "err", "cherry"})

	if first.Text != second.Text {
		t.Errorf("blob depends on input order:\n\t%q\n\t%q", first.Text, second.Text)
	}
}

func TestQuoteEscapesInvisibleRunes(t *testing.T) {
	inputs := []string{
		"\U0001F44D",            // a plain emoji
		"\U0001F44D\U0001F3FF",  // with a skin tone modifier
		"\U0001F1FA\U0001F1F8",  // regional indicators
		"❤️",                    // a variation selector
		"\U0001F468‍\U0001F466", // a zero-width joiner
		"\U0001F3F4\U000E0067",  // a tag character
		`say "hi"\`,             // quotes and backslashes
	}
	for _, in := range inputs {
		got := quote(in)

		back, err := strconv.Unquote(got)
		if err != nil {
			t.Errorf("quote(%q) = %s, which is not a Go string literal: %v", in, got, err)
			continue
		}
		if back != in {
			t.Errorf("quote(%q) round-tripped to %q", in, back)
		}
		// Nothing zero-width may survive into the literal, or it would sit
		// invisibly in the generated source.
		for _, r := range got {
			if isVariationSelector(r) || !strconv.IsPrint(r) {
				t.Errorf("quote(%q) = %s contains the invisible rune %U", in, got, r)
			}
		}
	}

	// Runes that do have a glyph stay legible rather than being escaped.
	if got, want := quote("\U0001F44D\U0001F3FF"), `"👍🏿"`; got != want {
		t.Errorf("quote of a skin-toned emoji = %s, want %s", got, want)
	}
}
