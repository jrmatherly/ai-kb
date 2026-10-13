package metric

import (
	"testing"

	"github.com/jrmatherly/ai-kb/internal/types"
	"github.com/stretchr/testify/require"
)

func TestSplitSentencesKeepsEverySentence(t *testing.T) {
	require.Equal(t, []string{"one", "two", "three", "four"}, splitSentences("one. two. three. four"))
	require.Equal(t, []string{"first", "second"}, splitSentences("first\u3002second\u3002"))
	require.Empty(t, splitSentences(" . . "))
}

// The second sentences differ, so a perfect score means the splitter dropped them.
func TestTextMetricsScoreEverySentence(t *testing.T) {
	input := &types.MetricInput{
		GeneratedTexts: "the cat sat on the mat. a dog ran in the park.",
		GeneratedGT:    "the cat sat on the mat. the bird flew over the house.",
	}
	require.InDelta(t, 0.6154, NewBLEUMetric(true, BLEU1Gram).Compute(input), 1e-4)
	require.InDelta(t, 0.4863, NewBLEUMetric(true, BLEU4Gram).Compute(input), 1e-4)
	require.InDelta(t, 0.5263, NewRougeMetric(true, "rouge-1", "f").Compute(input), 1e-4)
	require.InDelta(t, 0.5263, NewRougeMetric(true, "rouge-l", "f").Compute(input), 1e-4)
}
