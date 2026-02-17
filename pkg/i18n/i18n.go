package i18n

import (
	"embed"
	"encoding/json"
	"os"
	"sync"

	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

var (
	bundle      *i18n.Bundle
	localizer   *i18n.Localizer
	once        sync.Once
	currentLang string = "zh-CN"
)

//go:embed locales/*.json
var localeFS embed.FS

func Init() error {
	var initErr error
	once.Do(func() {
		bundle = i18n.NewBundle(language.Chinese)
		bundle.RegisterUnmarshalFunc("json", json.Unmarshal)

		dirEntries, err := localeFS.ReadDir("locales")
		if err != nil {
			initErr = err
			return
		}

		for _, entry := range dirEntries {
			if entry.IsDir() {
				continue
			}
			data, err := localeFS.ReadFile("locales/" + entry.Name())
			if err != nil {
				continue
			}
			bundle.MustParseMessageFileBytes(data, entry.Name())
		}

		lang := detectLanguage()
		currentLang = lang
		localizer = i18n.NewLocalizer(bundle, lang)
	})
	return initErr
}

func detectLanguage() string {
	lang := os.Getenv("MINDX_LANG")
	if lang != "" {
		return lang
	}

	envLang := os.Getenv("LANG")
	if envLang != "" {
		if len(envLang) >= 2 {
			prefix := envLang[:2]
			switch prefix {
			case "zh":
				return "zh-CN"
			case "en":
				return "en-US"
			}
		}
	}

	return "zh-CN"
}

func SetLanguage(lang string) {
	currentLang = lang
	if localizer != nil {
		localizer = i18n.NewLocalizer(bundle, lang)
	}
}

func GetLanguage() string {
	return currentLang
}

func T(id string) string {
	return TWithData(id, nil)
}

func TWithData(id string, templateData map[string]interface{}) string {
	if localizer == nil {
		return id
	}

	msg, err := localizer.Localize(&i18n.LocalizeConfig{
		MessageID:    id,
		TemplateData: templateData,
	})
	if err != nil {
		return id
	}
	return msg
}

func TWithDefault(id, defaultMessage string) string {
	return TWithDefaultAndData(id, defaultMessage, nil)
}

func TWithDefaultAndData(id, defaultMessage string, templateData map[string]interface{}) string {
	if localizer == nil {
		return defaultMessage
	}

	msg, err := localizer.Localize(&i18n.LocalizeConfig{
		MessageID: id,
		DefaultMessage: &i18n.Message{
			ID:    id,
			Other: defaultMessage,
		},
		TemplateData: templateData,
	})
	if err != nil {
		return defaultMessage
	}
	return msg
}

func SupportedLanguages() []string {
	return []string{"zh-CN", "en-US"}
}

func IsSupported(lang string) bool {
	for _, supported := range SupportedLanguages() {
		if supported == lang {
			return true
		}
	}
	return false
}
