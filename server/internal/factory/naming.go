package factory

import (
	"fmt"
	"math/rand"
	"strings"
	"unicode"
)

// ShipwrightPrefix derives a 2–4 char uppercase prefix from a civ
// name, used as the manufacturer stamp on every part serial. The civ
// itself is the manufacturer in this universe — no separate roster —
// so the prefix is a pure projection of civ.Name.
//
// Rules: take the first letter of each word (up to 4); strip non-letters;
// uppercase. Falls back to "GEN" if the name has nothing letter-shaped.
func ShipwrightPrefix(civName string) string {
	var b strings.Builder
	for _, word := range strings.Fields(civName) {
		for _, r := range word {
			if unicode.IsLetter(r) {
				b.WriteRune(unicode.ToUpper(r))
				break
			}
		}
		if b.Len() >= 4 {
			break
		}
	}
	if b.Len() == 0 {
		return "GEN"
	}
	if b.Len() == 1 {
		// Single-word civ names: pad to two letters using the first two
		// letters of the original name so the prefix doesn't collapse.
		for _, r := range civName {
			if unicode.IsLetter(r) {
				up := unicode.ToUpper(r)
				if up != rune(b.String()[0]) {
					b.WriteRune(up)
					break
				}
			}
		}
	}
	return b.String()
}

// PartSerial composes a part's serial number from the civ-derived
// manufacturer prefix, the archetype's short code, and a 4-digit batch
// number. Deterministic per rng.
func PartSerial(prefix, archetypeName string, rng *rand.Rand) string {
	return fmt.Sprintf("%s-%s-%04d", prefix, archetypeShortCode(archetypeName), rng.Intn(9000)+1000)
}

// archetypeShortCode squashes an archetype name down to a compact tag
// for serial numbers. Strategy: take the first letter of every
// alphabetic run (up to 4 letters) and uppercase. "Reaction Control
// Assembly (RCA)" → "RCA".
func archetypeShortCode(name string) string {
	var b strings.Builder
	prevLetter := false
	for _, r := range name {
		if unicode.IsLetter(r) {
			if !prevLetter {
				b.WriteRune(unicode.ToUpper(r))
				if b.Len() >= 4 {
					break
				}
			}
			prevLetter = true
		} else {
			prevLetter = false
		}
	}
	if b.Len() == 0 {
		return "GEN"
	}
	return b.String()
}
