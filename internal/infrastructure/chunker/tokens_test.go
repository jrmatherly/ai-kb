package chunker

import "testing"

func TestApproxTokenCount_English(t *testing.T) {
	got := ApproxTokenCount("The quick brown fox jumps over the lazy dog.", LangEnglish)
	// 44 chars / 4 ≈ 11 tokens
	if got < 9 || got > 13 {
		t.Errorf("English token estimate out of range: got %d, want 9..13", got)
	}
}

func TestApproxTokenCount_Chinese(t *testing.T) {
	got := ApproxTokenCount("\u8fd9\u662f\u4e00\u6bb5\u4e2d\u6587\u6d4b\u8bd5\u5185\u5bb9\u7528\u4e8e\u68c0\u9a8c\u5206\u8bcd\u4f30\u7b97", LangChinese)
	// 18 runes / 1.7 ≈ 10
	if got < 9 || got > 12 {
		t.Errorf("Chinese token estimate out of range: got %d, want 9..12", got)
	}
}

func TestApproxTokenCount_Empty(t *testing.T) {
	if got := ApproxTokenCount("", LangEnglish); got != 0 {
		t.Errorf("empty string should return 0 tokens, got %d", got)
	}
}

func TestApproxTokenCount_UnknownLang(t *testing.T) {
	got := ApproxTokenCount("Hello world hello world", "xx")
	if got <= 0 {
		t.Errorf("unknown lang should fall back to mixed, got %d", got)
	}
}

func TestDetectLanguage_English(t *testing.T) {
	if got := DetectLanguage("The quick brown fox jumps over the lazy dog."); got != LangEnglish {
		t.Errorf("expected English, got %s", got)
	}
}

func TestDetectLanguage_German(t *testing.T) {
	if got := DetectLanguage("Der schnelle braune Fuchs springt über den faulen Hund."); got != LangGerman {
		t.Errorf("expected German, got %s", got)
	}
}

func TestDetectLanguage_GermanByStopwords(t *testing.T) {
	// No umlauts but plenty of German function words.
	if got := DetectLanguage("Das ist ein Test und nicht mit Umlauten."); got != LangGerman {
		t.Errorf("expected German via stopwords, got %s", got)
	}
}

func TestDetectLanguage_Chinese(t *testing.T) {
	if got := DetectLanguage("\u8fd9\u662f\u4e00\u6bb5\u4e2d\u6587\u6d4b\u8bd5\u5185\u5bb9"); got != LangChinese {
		t.Errorf("expected Chinese, got %s", got)
	}
}

func TestDetectLanguage_Mixed(t *testing.T) {
	got := DetectLanguage("This \u8fd9\u662f mixed \u6d4b\u8bd5 content with \u591a\u8bed\u8a00 inside")
	if got != LangMixed {
		t.Errorf("expected Mixed, got %s", got)
	}
}

func TestCharsForTokenLimit_AppliesSafetyMargin(t *testing.T) {
	got := CharsForTokenLimit(1000, LangEnglish)
	// 1000 * 4 * 0.9 = 3600
	if got < 3500 || got > 3700 {
		t.Errorf("char budget for 1000 EN tokens out of range: got %d", got)
	}
}

func TestCharsForTokenLimit_ZeroTokens(t *testing.T) {
	if got := CharsForTokenLimit(0, LangEnglish); got != 0 {
		t.Errorf("zero tokens should give zero chars, got %d", got)
	}
}
