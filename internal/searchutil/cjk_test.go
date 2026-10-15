package searchutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Characters are spelled as escapes because tracked files are English only.
const (
	zhi, shi, ku = "\u77e5", "\u8bc6", "\u5e93" // Han: "knowledge base"
	ka, ta       = "\u30ab", "\u30bf"           // Katakana
	han, guk     = "\ud55c", "\uad6d"           // Hangul
	gu           = "\u80a1"                     // Han: "share"
	// "de-e-ta" (data) in Katakana; U+30FC is the long vowel mark.
	de, choon, taKana = "\u30c7", "\u30fc", "\u30bf"
)

func TestCJKBigrams(t *testing.T) {
	assert.Equal(t, zhi+shi+" "+shi+ku+" AI KB", CJKBigrams(zhi+shi+ku+"AI, KB"))
	assert.Equal(t, ka+ta+" "+han+guk, CJKBigrams(ka+ta+" "+han+guk))
	assert.Equal(t, "A "+gu, CJKBigrams("A"+gu), "a lone character stays a unigram")
	assert.Equal(t, "", CJKBigrams(" ,. "))
	assert.Equal(t, de+choon+" "+choon+taKana, CJKBigrams(de+choon+taKana),
		"the long vowel mark belongs to the Katakana run")
}

func TestCJKBigramsAndUnigrams(t *testing.T) {
	assert.Equal(t, zhi+shi+" "+shi+ku+" "+zhi+" "+shi+" "+ku+" base",
		CJKBigramsAndUnigrams(zhi+shi+ku+" base"))
	assert.Equal(t, gu, CJKBigramsAndUnigrams(gu), "a lone character is not repeated")
}
