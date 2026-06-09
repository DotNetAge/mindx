package i18n

import (
	"os"
	"strings"

	"golang.org/x/text/language"
)

var (
	// defaultFallback is used when no language can be detected.
	defaultFallback = language.Chinese

	// supported is the set of languages we have translations for.
	supported = map[language.Tag]struct{}{
		language.Chinese:          {},
		language.TraditionalChinese: {},
		language.English:          {},
	}
)

// ResolveTag converts a language string to a supported language.Tag.
// Resolution order:
//  1. Explicit lang string (from config or env var)
//  2. System locale (LANG / LC_ALL environment variables)
//  3. Default fallback (Chinese)
func ResolveTag(lang string) language.Tag {
	// 1. Explicit preference
	if lang != "" {
		if tag, err := language.Parse(lang); err == nil {
			if _, ok := supported[tag]; ok {
				return tag
			}
			// Try matching by base language or script
			if resolved := matchTag(tag); resolved != language.Und {
				return resolved
			}
		}
	}

	// 2. System locale
	if sys := SystemLocale(); sys != language.Und {
		if _, ok := supported[sys]; ok {
			return sys
		}
		if resolved := matchTag(sys); resolved != language.Und {
			return resolved
		}
	}

	// 3. Default fallback
	return defaultFallback
}

// SystemLocale detects the system language from environment variables.
// Returns language.Und if nothing can be determined.
func SystemLocale() language.Tag {
	// Check MINDX_LANG first (explicit override)
	if v := os.Getenv("MINDX_LANG"); v != "" {
		if tag, err := language.Parse(v); err == nil {
			return tag
		}
	}

	// Standard POSIX locale vars
	for _, key := range []string{"LANGUAGE", "LC_ALL", "LANG"} {
		if v := os.Getenv(key); v != "" {
			// Extract language part from e.g. "zh_CN.UTF-8" → "zh-CN"
			langPart := strings.Split(v, ".")[0]
			langPart = strings.ReplaceAll(langPart, "_", "-")
			if tag, err := language.Parse(langPart); err == nil {
				return tag
			}
		}
	}

	return language.Und
}

// matchTag attempts to match a language tag against our supported set.
// It checks script (Hans vs Hant) before falling back to base language,
// ensuring zh-HK/zh-Hant → TraditionalChinese, not Simplified.
func matchTag(t language.Tag) language.Tag {
	// Check script first: Hant → Traditional, Hans → Simplified
	if scr, conf := t.Script(); conf >= language.High {
		switch scr.String() {
		case "Hant":
			return language.TraditionalChinese
		case "Hans":
			return language.Chinese
		}
	}

	// Fallback: check region for known traditional regions
	switch t.String() {
	case "zh-TW", "zh-HK", "zh-MO":
		return language.TraditionalChinese
	}

	// Final fallback: base language — prefer Simplified as default for "zh"
	base, _ := t.Base()
	if base.String() == "zh" {
		return language.Chinese // default zh → simplified
	}

	return language.Und
}
