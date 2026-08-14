package generator

import (
	"strconv"
	"strings"
	"unicode"

	"emojis/internal/pkg/log"
)

// fold maps the non-ASCII letters that appear in CLDR emoji names onto ASCII.
// Names are otherwise ASCII, so an unmapped rune is dropped and logged rather
// than silently mangling an identifier when Unicode adds a new name.
var fold = map[rune]string{
	'Å': "A", 'å': "a",
	'ã': "a", 'á': "a", 'à': "a", 'â': "a", 'ä': "a",
	'ç': "c",
	'é': "e", 'è': "e", 'ê': "e", 'ë': "e",
	'í': "i", 'ì': "i", 'î': "i", 'ï': "i",
	'ñ': "n",
	'ô': "o", 'ó': "o", 'ò': "o", 'õ': "o", 'ö': "o", 'ø': "o",
	'ü': "u", 'ú': "u", 'ù': "u", 'û': "u",
	'ý': "y", 'ÿ': "y",
	'ß': "ss",
}

// symbols are name characters that carry meaning and so are spelled out rather
// than stripped. Without these, "keycap: #" and "keycap: *" would both reduce
// to Keycap.
var symbols = map[rune]string{
	'&': " and ",
	'#': " hash ",
	'*': " asterisk ",
	'+': " plus ",
	'-': " ",
}

// dropped characters would leave a stray word boundary if turned into a space:
// "man’s shoe" should be MansShoe, not ManSShoe.
const dropped = "'’`"

// numberWords lets an identifier that would otherwise start with a digit begin
// with a letter instead: "1st place medal" -> FirstPlaceMedal.
var numberWords = map[string]string{
	"1st": "First", "2nd": "Second", "3rd": "Third",
	"0": "Zero", "1": "One", "2": "Two", "3": "Three", "4": "Four",
	"5": "Five", "6": "Six", "7": "Seven", "8": "Eight", "9": "Nine",
}

// identSet turns CLDR names into unique exported Go identifiers.
type identSet struct {
	taken map[string]string // identifier -> the name that claimed it
}

func newIdentSet() *identSet { return &identSet{taken: make(map[string]string)} }

// add returns a unique identifier for name, suffixing a counter in the
// (currently unreached) event that two names reduce to the same identifier.
func (s *identSet) add(name string) string {
	base := ident(name)
	candidate := base
	for i := 2; ; i++ {
		if owner, clash := s.taken[candidate]; !clash {
			s.taken[candidate] = name
			if candidate != base {
				log.Info("identifier %q for %q collides with %q; using %q", base, name, owner, candidate)
			}
			return candidate
		}
		candidate = base + strconv.Itoa(i)
	}
}

// ident converts a CLDR name to an exported Go identifier:
// "thumbs up" -> ThumbsUp, "A button (blood type)" -> AButtonBloodType.
func ident(name string) string {
	var sb strings.Builder
	for _, r := range name {
		switch {
		case strings.ContainsRune(dropped, r):
			// no separator: elide entirely
		case r < unicode.MaxASCII && (unicode.IsLetter(r) || unicode.IsDigit(r)):
			sb.WriteRune(r)
		case symbols[r] != "":
			sb.WriteString(symbols[r])
		case fold[r] != "":
			sb.WriteString(fold[r])
		case unicode.IsPunct(r) || unicode.IsSpace(r):
			sb.WriteByte(' ')
		case r > unicode.MaxASCII:
			// A letter with no folding would silently vanish from the
			// identifier, so make the omission visible.
			log.Error("no ASCII folding for %q in emoji name %q; dropping it", r, name)
		default:
			sb.WriteByte(' ')
		}
	}

	words := strings.Fields(sb.String())
	for i, w := range words {
		if i == 0 {
			if spelled, ok := numberWords[w]; ok {
				w = spelled
			} else if w[0] >= '0' && w[0] <= '9' {
				// Digits cannot start an identifier and there is no obvious
				// English word for this one.
				w = "N" + w
			}
		}
		// Upper-case the first rune only, so existing capitalisation survives:
		// "ID button" -> IDButton, not IdButton.
		words[i] = strings.ToUpper(w[:1]) + w[1:]
	}
	return strings.Join(words, "")
}
