package generator

// A VariantDef is one value of the generated Variant type.
type VariantDef struct {
	Ident string // Go identifier, e.g. "DarkSkinTone"
	Token string // CLDR name token, e.g. "dark skin tone"
}

// SkinTones and HairStyles are the closed set of CLDR name tokens treated as
// runtime modifiers rather than as part of an emoji's function name.
//
// Every other token that can follow the colon in a CLDR name ("man", "woman",
// "boy", "girl", "adult", "child", a flag's country, a keycap's digit) selects
// a genuinely different emoji rather than a restyling of one, so those become
// part of the generated function name instead: family: man, boy -> FamilyManBoy.
//
// Declaration order here is the order of the generated iota constants; skin
// tones must stay contiguous and first so the generated firstSkinTone and
// lastSkinTone bounds hold.
var (
	SkinTones = []VariantDef{
		{"LightSkinTone", "light skin tone"},
		{"MediumLightSkinTone", "medium-light skin tone"},
		{"MediumSkinTone", "medium skin tone"},
		{"MediumDarkSkinTone", "medium-dark skin tone"},
		{"DarkSkinTone", "dark skin tone"},
	}
	HairStyles = []VariantDef{
		{"RedHair", "red hair"},
		{"CurlyHair", "curly hair"},
		{"WhiteHair", "white hair"},
		{"BlondHair", "blond hair"},
		{"Bald", "bald"},
		{"Beard", "beard"},
	}
)

// Variants is every modifier, skin tones first.
func Variants() []VariantDef {
	return append(append([]VariantDef{}, SkinTones...), HairStyles...)
}

// variantIdent returns the Go identifier for a CLDR token, and whether the
// token is a modifier at all.
func variantIdent(token string) (string, bool) {
	for _, v := range Variants() {
		if v.Token == token {
			return v.Ident, true
		}
	}
	return "", false
}
