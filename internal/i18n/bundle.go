package i18n

import (
	"embed"
	"encoding/json"
	"fmt"
	"strings"

	"golang.org/x/text/language"
)

//go:embed locales/*.json
var localeFS embed.FS

// translations holds the loaded key-value map for the active locale.
var translations map[string]string

var (
	tag language.Tag
)

// Init initializes the i18n bundle with the given language tag string.
// lang should be a BCP 47 tag like "zh", "en", "zh-TW", etc.
// Falls back to system locale if lang is empty, then to "zh".
func Init(lang string) error {
	tag = ResolveTag(lang)

	// Map tag to translation file.
	//   zh / zh-CN          → zh.json  (Simplified Chinese)
	//   zh-TW / zh-HK / zh-Hant → zh-TW.json  (Traditional Chinese)
	//   en                  → en.json
	fileName := localeFileName(tag)

	data, err := localeFS.ReadFile("locales/" + fileName)
	if err != nil {
		// Fallback to zh
		data, err = localeFS.ReadFile("locales/zh.json")
		if err != nil {
			return fmt.Errorf("i18n: failed to load default locale: %w", err)
		}
	}

	var m map[string]string
	if err := json.Unmarshal(data, &m); err != nil {
		return fmt.Errorf("i18n: failed to parse %s: %w", fileName, err)
	}
	translations = m

	return nil
}

// localeFileName maps a language.Tag to its JSON translation filename.
func localeFileName(t language.Tag) string {
	// Check for traditional Chinese variants first (zh-TW, zh-HK, zh-Hant, etc.)
	if strings.HasPrefix(t.String(), "zh-TW") ||
		strings.HasPrefix(t.String(), "zh-HK") ||
		strings.HasPrefix(t.String(), "zh-Hant") {
		return "zh-TW.json"
	}
	base, _ := t.Base()
	return base.String() + ".json"
}

// T translates a message key with optional arguments.
// It is the primary entry point for all user-facing strings.
//
// Usage:
//
//	i18n.T("client.status.idle")                      // simple
//	i18n.T("client.notify.connected", provider)        // with args (fmt.Sprintf style)
func T(key string, args ...any) string {
	if translations == nil {
		if len(args) == 0 {
			return key
		}
		return fmt.Sprintf(key, args...)
	}

	msg, ok := translations[key]
	if !ok {
		// Key not found: return key itself (or formatted with args)
		if len(args) == 0 {
			return key
		}
		return fmt.Sprintf(key, args...)
	}

	if len(args) == 0 {
		return msg
	}
	return fmt.Sprintf(msg, args...)
}

// Lang returns the current language tag.
func Lang() language.Tag {
	return tag
}

// IsChinese reports whether the current language is Chinese (any variant: simplified or traditional).
func IsChinese() bool {
	base, _ := tag.Base()
	return base.String() == "zh"
}

// MustInit calls Init and panics on failure.
func MustInit(lang string) {
	if err := Init(lang); err != nil {
		panic(err)
	}
}

// SwitchLanguage reinitializes the i18n bundle with a new language tag.
// It is safe to call at runtime after the initial Init.
func SwitchLanguage(lang string) error {
	return Init(lang)
}

// SupportedLanguages returns all supported language tags.
func SupportedLanguages() []language.Tag {
	return []language.Tag{
		language.Chinese,
		language.TraditionalChinese,
		language.English,
	}
}

// LanguageName returns a human-readable name for a language tag.
func LanguageName(t language.Tag) string {
	base, _ := t.Base()
	switch base.String() {
	case "zh":
		if strings.HasPrefix(t.String(), "zh-TW") || strings.HasPrefix(t.String(), "zh-HK") {
			return "繁體中文"
		}
		return "简体中文"
	case "en":
		return "English"
	default:
		return t.String()
	}
}
