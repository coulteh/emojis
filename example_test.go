package emojis_test

import (
	"fmt"

	"emojis"
)

func Example() {
	fmt.Println(emojis.ThumbsUp())
	fmt.Println(emojis.PartyPopper())
	fmt.Println(emojis.FamilyManBoy())
	// Output:
	// 👍
	// 🎉
	// 👨‍👦
}

// Emoji that can be restyled take the styling as an argument rather than
// having a function of their own.
func Example_variants() {
	fmt.Println(emojis.ThumbsUp(emojis.DarkSkinTone))
	fmt.Println(emojis.WavingHand(emojis.MediumLightSkinTone))

	// A skin tone and a hair style describe one person, so either order works.
	fmt.Println(emojis.Person(emojis.MediumSkinTone, emojis.RedHair))
	fmt.Println(emojis.Person(emojis.RedHair, emojis.MediumSkinTone))

	// Two skin tones describe one person each, so the order is the meaning.
	fmt.Println(emojis.MenHoldingHands(emojis.LightSkinTone, emojis.DarkSkinTone))
	fmt.Println(emojis.MenHoldingHands(emojis.DarkSkinTone, emojis.LightSkinTone))
	// Output:
	// 👍🏿
	// 👋🏼
	// 🧑🏽‍🦰
	// 🧑🏽‍🦰
	// 👨🏻‍🤝‍👨🏿
	// 👨🏿‍🤝‍👨🏻
}

// Combinations Unicode does not define return the empty string.
func Example_unsupported() {
	fmt.Printf("%q\n", emojis.GrinningFace(emojis.DarkSkinTone))
	fmt.Printf("%q\n", emojis.ThumbsUp(emojis.RedHair))
	// Output:
	// ""
	// ""
}

// Lookup reaches an emoji whose name is only known at run time.
func ExampleLookup() {
	fmt.Println(emojis.Lookup("thumbs up"))
	fmt.Println(emojis.Lookup("thumbs up", emojis.DarkSkinTone))
	fmt.Println(emojis.Lookup("family: man, boy"))
	fmt.Printf("%q\n", emojis.Lookup("not an emoji"))
	// Output:
	// 👍
	// 👍🏿
	// 👨‍👦
	// ""
}

func ExampleNames() {
	names := emojis.Names()
	fmt.Println(len(names) > 1000)
	fmt.Println(emojis.Lookup(names[0]) != "")
	// Output:
	// true
	// true
}

func ExampleVariant_String() {
	fmt.Println(emojis.DarkSkinTone)
	fmt.Println(emojis.RedHair)
	// Output:
	// dark skin tone
	// red hair
}

func ExampleParseVariant() {
	v, ok := emojis.ParseVariant("medium-dark skin tone")
	fmt.Println(v, ok)
	fmt.Println(emojis.ThumbsUp(v))
	// Output:
	// medium-dark skin tone true
	// 👍🏾
}
