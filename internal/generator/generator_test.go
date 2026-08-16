package generator

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
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
	var logged bytes.Buffer
	slog.SetDefault(slog.New(slog.NewTextHandler(&logged, nil)))
	defer slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))

	s := newIdentSet()
	// Two names that reduce to the same identifier, as "turkey" and "Turkey"
	// did in emoji 5.0.
	if got := s.add("red heart"); got != "RedHeart" {
		t.Fatalf("first add = %q, want RedHeart", got)
	}
	if got := s.add("red (heart)"); got != "RedHeart2" {
		t.Errorf("colliding add = %q, want RedHeart2", got)
	}
	if got := s.add("red -heart-"); got != "RedHeart3" {
		t.Errorf("second colliding add = %q, want RedHeart3", got)
	}

	// The warning has to name whoever holds the identifier that was wanted,
	// or it says nothing useful about the collision.
	if !strings.Contains(logged.String(), `claimed_by="red heart"`) {
		t.Errorf("collision warning does not name the holder:\n%s", logged.String())
	}
	if strings.Contains(logged.String(), `claimed_by=""`) {
		t.Errorf("collision warning reports an empty holder:\n%s", logged.String())
	}
}

// A letter with no ASCII folding would vanish from the identifier without
// trace, so ident says so. Nothing in Unicode's names needs this today; it is
// there for the release that does.
func TestIdentReportsUnfoldableRunes(t *testing.T) {
	var logged bytes.Buffer
	slog.SetDefault(slog.New(slog.NewTextHandler(&logged, nil)))
	defer slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))

	if got, want := ident("tanabata \u77ed\u518a"), "Tanabata"; got != want {
		t.Errorf("ident with unfoldable runes = %q, want %q", got, want)
	}
	if !strings.Contains(logged.String(), "no ASCII folding") {
		t.Errorf("dropping a rune went unreported:\n%s", logged.String())
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
		name, variants, _, err := split(tt.cldr)
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

func TestSplitRejectsMalformedNames(t *testing.T) {
	tests := []struct{ name, cldr, want string }{
		{
			"more modifiers than the lookup key holds",
			"crowd: light skin tone, dark skin tone, medium skin tone",
			"more than the supported",
		},
		{
			// One colon separates the name from its qualifiers; a second one
			// means the line is not shaped the way this reads it.
			"two colon separators",
			"keycap: 1: light skin tone",
			"more than one",
		},
	}
	for _, tt := range tests {
		_, _, _, err := split(tt.cldr)
		if err == nil {
			t.Errorf("%s: split(%q) succeeded, want an error", tt.name, tt.cldr)
			continue
		}
		if !strings.Contains(err.Error(), tt.want) {
			t.Errorf("%s: split failed with %q, want that to mention %q", tt.name, err, tt.want)
		}
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

// Unicode added the emoji version to each line in 12.1; rows from before that
// carry only the emoji and its name.
func TestParseWithoutEmojiVersion(t *testing.T) {
	const old = "# Version: 11.0\n# group: Smileys & Emotion\n" +
		"1F600 ; fully-qualified # \U0001F600 grinning face\n" +
		"1F44D 1F3FF ; fully-qualified # \U0001F44D\U0001F3FF thumbs up: dark skin tone\n"

	ds, err := Parse(strings.NewReader(old), "sample")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(ds.Emojis) != 2 {
		t.Fatalf("parsed %d emoji, want 2", len(ds.Emojis))
	}
	if got := ds.Emojis[0].Name; got != "grinning face" {
		t.Errorf("name = %q, want %q", got, "grinning face")
	}
	if got := ds.Emojis[1].Name; got != "thumbs up: dark skin tone" {
		t.Errorf("name = %q, want %q", got, "thumbs up: dark skin tone")
	}
}

func TestParseRejectsBadData(t *testing.T) {
	// Each case names the error it expects. Asserting only that Parse failed
	// lets a fixture drift onto some other error and keep passing: two of
	// these were doing exactly that, one of them reaching the group check
	// before the version check it was written for.
	tests := []struct{ name, data, want string }{
		{
			"no version header",
			"# group: X\n1F600 ; fully-qualified # \U0001F600 E1.0 grinning face\n",
			"no version header",
		},
		{
			"no emoji at all",
			"# Version: 17.0\n# group: X\n",
			"no fully-qualified emoji",
		},
		{
			"missing status separator",
			"# Version: 17.0\n# group: X\n1F600 fully-qualified # \U0001F600 E1.0 grinning face\n",
			`no ";" separator`,
		},
		{
			"missing comment separator",
			"# Version: 17.0\n# group: X\n1F600 ; fully-qualified \U0001F600 E1.0 grinning face\n",
			`no "#" separator`,
		},
		{
			// A name is required; the emoji on its own does not parse.
			"unparseable comment",
			"# Version: 17.0\n# group: X\n1F600 ; fully-qualified # \U0001F600\n",
			"cannot parse comment",
		},
		{
			"bad code point",
			"# Version: 17.0\n# group: X\nZZZZ ; fully-qualified # \U0001F600 E1.0 grinning face\n",
			"bad code point",
		},
		// The comment repeats the sequence, so a mismatch means a misread row.
		{
			"comment disagrees with code points",
			"# Version: 17.0\n# group: X\n1F600 ; fully-qualified # \U0001F601 E1.0 grinning face\n",
			"comment shows",
		},
		{
			"emoji before any group",
			"# Version: 17.0\n1F600 ; fully-qualified # \U0001F600 E1.0 grinning face\n",
			"before any group header",
		},
	}
	for _, tt := range tests {
		_, err := Parse(strings.NewReader(tt.data), "sample")
		if err == nil {
			t.Errorf("%s: Parse succeeded, want an error", tt.name)
			continue
		}
		if !strings.Contains(err.Error(), tt.want) {
			t.Errorf("%s: Parse failed with %q, want that to mention %q", tt.name, err, tt.want)
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

// Two emoji claiming the same name and the same variants would silently lose
// one of them, so Build refuses instead.
func TestBuildRejectsDuplicateForms(t *testing.T) {
	dup := "# Version: 17.0\n# group: X\n" +
		"1F44D 1F3FB ; fully-qualified # \U0001F44D\U0001F3FB E1.0 thumbs up: light skin tone\n" +
		"1F44E 1F3FB ; fully-qualified # \U0001F44E\U0001F3FB E1.0 thumbs up: light skin tone\n"

	ds, err := Parse(strings.NewReader(dup), "sample")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if _, err := Build(ds); err == nil {
		t.Error("Build accepted two emoji with the same name and variants")
	} else if !strings.Contains(err.Error(), "two forms for") {
		t.Errorf("Build failed with %q, want that to mention the duplicate forms", err)
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
		{"FamilyManBoy", "\U0001F468\u200D\U0001F466"},
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

// The generated blobs are tens of thousands of characters. Nothing else in
// these tests is long enough to make quoteChunked break a line, so the split
// itself is exercised directly.
func TestQuoteChunkedSplitsLongStrings(t *testing.T) {
	long := strings.Repeat("\U0001F600", 200)
	lit := quoteChunked(long)

	lines := strings.Split(lit, "\n")
	if len(lines) < 3 {
		t.Fatalf("quoteChunked put %d bytes on %d line(s); it should split", len(long), len(lines))
	}
	for i, line := range lines {
		// A chunk may overshoot by the last escape it fitted, never more.
		if len(line) > chunkWidth+16 {
			t.Errorf("line %d is %d bytes, past the chunk width of %d", i+1, len(line), chunkWidth)
		}
	}

	// Whatever the split, the constant still has to hold the original string.
	var got strings.Builder
	for _, part := range regexp.MustCompile(`"(?:[^"\\]|\\.)*"`).FindAllString(lit, -1) {
		s, err := strconv.Unquote(part)
		if err != nil {
			t.Fatalf("chunk %s is not a Go string literal: %v", part, err)
		}
		got.WriteString(s)
	}
	if got.String() != long {
		t.Error("the chunks do not reassemble to the original string")
	}

	if q := quoteChunked(""); q != `""` {
		t.Errorf(`quoteChunked("") = %s, want ""`, q)
	}
}

// How a function's variants are described depends on their shape, and the
// shapes read differently to a caller: two skin tones are one per figure,
// where the order is the meaning, while a tone and a hair style describe one
// figure and commute.
func TestDocDescribesTheVariantShape(t *testing.T) {
	form := func(v ...string) Form { return Form{Variants: v, Sequence: "x"} }

	tests := []struct {
		name string
		base *Base
		want string
	}{
		{
			"a skin tone per figure",
			&Base{Ident: "MenHoldingHands", Name: "men holding hands", Plain: "\U0001F46C",
				Forms: []Form{form("LightSkinTone"), form("LightSkinTone", "DarkSkinTone")}},
			"one skin tone for both figures",
		},
		{
			"a skin tone and a hair style",
			&Base{Ident: "Person", Name: "person", Plain: "\U0001F9D1",
				Forms: []Form{form("MediumSkinTone"), form("MediumSkinTone", "RedHair")}},
			"in either order",
		},
		{
			"a single variant",
			&Base{Ident: "ThumbsUp", Name: "thumbs up", Plain: "\U0001F44D",
				Forms: []Form{form("DarkSkinTone")}},
			"It accepts one variant",
		},
		{
			// Unicode defines a couple of emoji only in modified forms, so
			// there is no unmodified one to show or to fall back to.
			"no unmodified form",
			&Base{Ident: "KissPersonPerson", Name: "kiss: person, person",
				Forms: []Form{form("LightSkinTone", "DarkSkinTone")}},
			"only in modified forms",
		},
	}
	for _, tt := range tests {
		got := strings.Join(doc(tt.base), " ")
		if !strings.Contains(got, tt.want) {
			t.Errorf("%s: doc reads %q, want that to mention %q", tt.name, got, tt.want)
		}
	}
}

func TestQuoteEscapesInvisibleRunes(t *testing.T) {
	inputs := []string{
		"\U0001F44D",                 // a plain emoji
		"\U0001F44D\U0001F3FF",       // with a skin tone modifier
		"\U0001F1FA\U0001F1F8",       // regional indicators
		"❤\uFE0F",                    // a variation selector
		"\U0001F468\u200D\U0001F466", // a zero-width joiner
		"\U0001F3F4\U000E0067",       // a tag character
		`say "hi"\`,                  // quotes and backslashes
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

// A qualifier the generator has no modifier for silently becomes part of a
// function name. That is fine for a flag or a keycap, which belong to one
// emoji each, but a styling applies across many: emoji 12.0 introduced six
// hair styles at once, each turning up under man, woman and person.
func TestBuildWarnsAboutUnrecognisedStylings(t *testing.T) {
	const data = "# Version: 18.0\n# group: People & Body\n" +
		// A styling Unicode has not invented yet, shared by three emoji.
		"1F9D1 200D 1F9B0 ; fully-qualified # \U0001F9D1\u200D\U0001F9B0 E1.0 person: teal hair\n" +
		"1F468 200D 1F9B0 ; fully-qualified # \U0001F468\u200D\U0001F9B0 E1.0 man: teal hair\n" +
		"1F469 200D 1F9B0 ; fully-qualified # \U0001F469\u200D\U0001F9B0 E1.0 woman: teal hair\n" +
		// A flag qualifier belongs to one emoji and must stay quiet.
		"1F1FA 1F1F8 ; fully-qualified # \U0001F1FA\U0001F1F8 E2.0 flag: United States\n" +
		"1F1EC 1F1E7 ; fully-qualified # \U0001F1EC\U0001F1E7 E2.0 flag: United Kingdom\n" +
		// "man" names a figure under two different emoji. It is shared exactly
		// the way a styling is, and is only quiet because PersonWords says so.
		"1F468 200D 1F466 ; fully-qualified # \U0001F468\u200D\U0001F466 E4.0 family: man, boy\n" +
		"1F468 200D 2764 FE0F 200D 1F48B 200D 1F468 ; fully-qualified # \U0001F468\u200D❤\uFE0F\u200D\U0001F48B\u200D\U0001F468 E2.0 kiss: man, man\n"

	var logged bytes.Buffer
	slog.SetDefault(slog.New(slog.NewTextHandler(&logged, nil)))
	defer slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))

	ds, err := Parse(strings.NewReader(data), "sample")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if _, err := Build(ds); err != nil {
		t.Fatalf("Build: %v", err)
	}

	out := logged.String()
	if !strings.Contains(out, `token="teal hair"`) {
		t.Errorf("a styling shared by three emoji went unreported:\n%s", out)
	}
	// Each country belongs to one emoji, so no flag is shared.
	if strings.Contains(out, "United States") || strings.Contains(out, "United Kingdom") {
		t.Errorf("a flag qualifier was reported as a styling:\n%s", out)
	}
	if strings.Contains(out, `token=man`) {
		t.Errorf("a word naming a figure was reported as a styling:\n%s", out)
	}
}

// The real data must not trip the warning, or it is noise rather than signal.
func TestSampleDataHasNoUnrecognisedStylings(t *testing.T) {
	var logged bytes.Buffer
	slog.SetDefault(slog.New(slog.NewTextHandler(&logged, nil)))
	defer slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))

	if _, err := Build(parseSample(t)); err != nil {
		t.Fatalf("Build: %v", err)
	}
	if strings.Contains(logged.String(), "unrecognised qualifier") {
		t.Errorf("the sample data tripped the styling warning:\n%s", logged.String())
	}
}

func TestWriteFilesOnlyWritesWhatChanged(t *testing.T) {
	dir := t.TempDir()
	files := []file{
		{Name: "a_gen.go", Contents: []byte("package emojis // a\n")},
		{Name: "b_gen.go", Contents: []byte("package emojis // b\n")},
	}

	changed, err := writeFiles(dir, files)
	if err != nil {
		t.Fatalf("writeFiles: %v", err)
	}
	if changed != 2 {
		t.Errorf("first run changed %d files, want 2", changed)
	}

	// Nothing has moved, so a second run must write nothing.
	changed, err = writeFiles(dir, files)
	if err != nil {
		t.Fatalf("writeFiles: %v", err)
	}
	if changed != 0 {
		t.Errorf("second run changed %d files, want 0", changed)
	}

	files[1].Contents = []byte("package emojis // b, revised\n")
	changed, err = writeFiles(dir, files)
	if err != nil {
		t.Fatalf("writeFiles: %v", err)
	}
	if changed != 1 {
		t.Errorf("changed %d files after editing one, want 1", changed)
	}
	got, err := os.ReadFile(filepath.Join(dir, "b_gen.go"))
	if err != nil || string(got) != string(files[1].Contents) {
		t.Errorf("b_gen.go = %q, %v; want the revised contents", got, err)
	}
}

func TestChangedFunctions(t *testing.T) {
	m, err := Build(parseSample(t))
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	// Generated from the same data: nothing moved.
	files, err := render(m, "emojis")
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	current := files[2].Contents
	if added, removed := changedFunctions(current, m); len(added) > 0 || len(removed) > 0 {
		t.Errorf("regenerating unchanged data reported added=%v removed=%v", added, removed)
	}

	// A previous file naming one emoji this release does not, and missing
	// several it does.
	previous := []byte("package emojis\n\nfunc ThumbsUp(v ...Variant) string { return \"\" }\n" +
		"func RetiredEmoji() string { return \"\" }\n")
	added, removed := changedFunctions(previous, m)
	if len(added) != len(m.Bases)-1 {
		t.Errorf("added %d functions, want %d", len(added), len(m.Bases)-1)
	}
	if len(removed) != 1 || removed[0] != "RetiredEmoji" {
		t.Errorf("removed = %v, want [RetiredEmoji]", removed)
	}
	// Sorted, so a report reads the same way twice.
	for i := 1; i < len(added); i++ {
		if added[i-1] >= added[i] {
			t.Fatalf("added is not sorted: %q before %q", added[i-1], added[i])
		}
	}
}

func TestSummarise(t *testing.T) {
	short := []string{"Tent", "Fountain"}
	if got, want := summarise(short), "Tent, Fountain"; got != want {
		t.Errorf("summarise(%v) = %q, want %q", short, got, want)
	}

	long := make([]string, listedNames+5)
	for i := range long {
		long[i] = fmt.Sprintf("Emoji%02d", i)
	}
	got := summarise(long)
	if !strings.HasSuffix(got, "and 5 more") {
		t.Errorf("summarise of %d names = %q, want it to summarise the tail", len(long), got)
	}
	if strings.Count(got, ",") != listedNames-1 {
		t.Errorf("summarise listed %d names, want %d", strings.Count(got, ",")+1, listedNames)
	}
}

func TestReportFunctionChanges(t *testing.T) {
	m, err := Build(parseSample(t))
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	capture := func(previous string) string {
		t.Helper()
		dir := t.TempDir()
		if previous != "" {
			if err := os.WriteFile(filepath.Join(dir, emojiFile), []byte(previous), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		var logged bytes.Buffer
		slog.SetDefault(slog.New(slog.NewTextHandler(&logged, &slog.HandlerOptions{Level: slog.LevelDebug})))
		defer slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
		reportFunctionChanges(dir, m)
		return logged.String()
	}

	// Nothing to compare against on a first run.
	if got := capture(""); !strings.Contains(got, "nothing to compare against") {
		t.Errorf("a missing previous file was not reported:\n%s", got)
	}

	// The file it is about to write: no emoji moved.
	files, err := render(m, "emojis")
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if got := capture(string(files[2].Contents)); !strings.Contains(got, "same emoji as last time") {
		t.Errorf("an unchanged release was not reported:\n%s", got)
	}

	// A previous release with one emoji this one drops, and missing the rest.
	got := capture("package emojis\n\nfunc RetiredEmoji() string { return \"\" }\n")
	if !strings.Contains(got, "emoji added") {
		t.Errorf("new emoji were not reported:\n%s", got)
	}
	if !strings.Contains(got, "level=WARN") || !strings.Contains(got, "RetiredEmoji") {
		t.Errorf("a removed function was not warned about:\n%s", got)
	}
}
