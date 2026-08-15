# emojis

A Go library that allows emoji to be used in a user-friendly way.

Every emoji Unicode publishes has a function named after it, generated from
Unicode's own data. Emoji that can be restyled — with a skin tone, a hair
style, or both — take the styling as an argument instead of having a function
per combination.

```go
import "github.com/coulteh/emojis"

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

Only emoji that can be restyled take an argument at all. An emoji Unicode
defines no variants for has no parameter, so asking for one is a compile error
rather than a surprise at run time:

```go
emojis.GrinningFace()                    // 😀
emojis.GrinningFace(emojis.DarkSkinTone) // does not compile
```

Each function's doc comment lists the variants it accepts. For an emoji that
does take variants, a combination Unicode does not define returns the empty
string:

```go
emojis.ThumbsUp(emojis.RedHair) // "" — no hair on a hand
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

## Emoji data

Everything here is generated from Unicode's [emoji-test.txt], currently emoji
17.0: **1,898 functions** covering 3,944 emoji. When Unicode publishes a new
release, pick it up with:

```sh
go generate ./...
```

[emoji-test.txt]: https://www.unicode.org/Public/emoji/latest/emoji-test.txt
