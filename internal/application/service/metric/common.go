package metric

import (
	"regexp"
	"strings"

	"github.com/jrmatherly/ai-kb/internal/types"
)

func sum(m map[string]int) int {
	s := 0
	for _, v := range m {
		s += v
	}
	return s
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func abs(a int) int {
	if a < 0 {
		return -a
	}
	return a
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// A period ends a sentence only before whitespace or the end of the text, so
// decimals and version numbers stay whole.
var sentenceSeparator = regexp.MustCompile(`\x{3002}|\.(?:\s|$)`)

// splitSentences splits on the ideographic full stop or a sentence-ending
// period and drops empty sentences.
func splitSentences(text string) []string {
	var sentences []string
	for _, s := range sentenceSeparator.Split(text, -1) {
		if s = strings.TrimSpace(s); s != "" {
			sentences = append(sentences, s)
		}
	}
	return sentences
}

func splitIntoWords(sentences []string) []string {
	// Matches a Chinese run, an English run or a punctuation character.
	re := regexp.MustCompile(`([\p{Han}]+)|([a-zA-Z0-9_.,!?]+)|(\p{P})`)

	var tokens []string
	for _, text := range sentences {
		matches := re.FindAllStringSubmatch(text, -1)

		for _, groups := range matches {
			chineseBlock := groups[1]
			englishBlock := groups[2]
			punctuation := groups[3]

			switch {
			case chineseBlock != "":
				words := types.Jieba.Cut(chineseBlock, true)
				tokens = append(tokens, words...)
			case englishBlock != "":
				engTokens := strings.Fields(englishBlock)
				tokens = append(tokens, engTokens...)
			case punctuation != "":
				tokens = append(tokens, punctuation)
			}
		}
	}
	return tokens
}

func ToSet[T comparable](li []T) map[T]struct{} {
	res := make(map[T]struct{}, len(li))
	for _, v := range li {
		res[v] = struct{}{}
	}
	return res
}

func SliceMap[T any, Y any](li []T, fn func(T) Y) []Y {
	res := make([]Y, len(li))
	for i, v := range li {
		res[i] = fn(v)
	}
	return res
}

func Hit[T comparable](li []T, set map[T]struct{}) int {
	count := 0
	for _, v := range li {
		if _, exist := set[v]; exist {
			count++
		}
	}
	return count
}

func Fold[T any, Y any](slice []T, initial Y, f func(Y, T) Y) Y {
	accumulator := initial
	for _, item := range slice {
		accumulator = f(accumulator, item)
	}
	return accumulator
}
