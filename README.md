# emojis

A Go library that allows emoji to be used in a user-friendly way.

Every emoji Unicode publishes has a function named after it, generated from
Unicode's own data. Emoji that can be restyled — with a skin tone, a hair
style, or both — take the styling as an argument instead of having a function
per combination.

```go
import "emojis"

emojis.ThumbsUp()     // 👍
emojis.PartyPopper()  // 🎉
emojis.FamilyManBoy() // 👨‍👦

emojis.ThumbsUp(emojis.DarkSkinTone)                  // 👍🏿
emojis.Person(emojis.MediumSkinTone, emojis.RedHair)  // 🧑🏽‍🦰
```

## Variants

`Variant` covers the five skin tones and the hair styles:

| | |
|---|---|
| Skin tones | `LightSkinTone`, `MediumLightSkinTone`, `MediumSkinTone`, `MediumDarkSkinTone`, `DarkSkinTone` |
| Hair styles | `RedHair`, `CurlyHair`, `WhiteHair`, `BlondHair`, `Bald`, `Beard` |

A skin tone and a hair style describe the same person, so they may be passed in
either order:

```go
emojis.Person(emojis.MediumSkinTone, emojis.RedHair) // 🧑🏽‍🦰
emojis.Person(emojis.RedHair, emojis.MediumSkinTone) // 🧑🏽‍🦰 — the same emoji
```

Two skin tones describe one person each, so there the order is the meaning:

```go
emojis.MenHoldingHands(emojis.LightSkinTone)                     // 👬🏻 both figures
emojis.MenHoldingHands(emojis.LightSkinTone, emojis.DarkSkinTone) // 👨🏻‍🤝‍👨🏿
emojis.MenHoldingHands(emojis.DarkSkinTone, emojis.LightSkinTone) // 👨🏿‍🤝‍👨🏻
```

Each function's doc comment lists the variants it accepts. A combination
Unicode does not define returns the empty string:

```go
emojis.GrinningFace(emojis.DarkSkinTone) // "" — no skin tone for this emoji
emojis.ThumbsUp(emojis.RedHair)          // "" — no hair on a hand
```

### What is not a variant

Only skin tones and hair styles restyle an emoji. Every other qualifier in a
Unicode name selects a genuinely different emoji, so it becomes part of the
function name:

```go
emojis.FamilyManBoy()      // 👨‍👦
emojis.KissWomanMan()      // 👩‍❤️‍💋‍👨
emojis.FlagUnitedStates()  // 🇺🇸
emojis.Keycap1()           // 1️⃣
```

## Lookup by name

When the emoji is only known at run time, `Lookup` takes the Unicode name that
the generated functions are named after:

```go
emojis.Lookup("thumbs up")                     // 👍
emojis.Lookup("thumbs up", emojis.DarkSkinTone) // 👍🏿
emojis.Lookup("family: man, boy")              // 👨‍👦
emojis.Lookup("not an emoji")                  // ""
```

Names are Unicode's own and are matched exactly, so a handful are capitalised:
`Lookup("T-Rex")`, not `Lookup("t-rex")`. `Names()` returns every name `Lookup`
accepts, sorted.

## Regenerating

```sh
go generate ./...
```

This fetches [emoji-test.txt] from Unicode and rewrites `emoji_gen.go`,
`tables_gen.go` and `variants_gen.go`. The generator is in `internal/`, and
takes flags for the source, the output directory and the package name:

```sh
go run ./internal --source ./emoji-test.txt --out . --package emojis
```

`emoji-test.txt` is used in preference to `emoji-sequences.txt` because it
names every emoji individually. `emoji-sequences.txt` collapses runs of code
points into ranges, naming only the endpoints:

```
2648..2653 ; Basic_Emoji ; Aries..Pisces
```

which leaves the ten signs between Aries and Pisces without names.

The generated package tracks Unicode's `latest` release, currently emoji 17.0:
**1,898 functions**, 314 of which take variants, covering 3,944 emoji in total.
Only fully-qualified emoji are generated; the minimally-qualified and
unqualified rows are the same emoji missing presentation selectors, and the
component rows are the bare skin tone and hair modifiers, which are not emoji
in their own right.

[emoji-test.txt]: https://www.unicode.org/Public/emoji/latest/emoji-test.txt
