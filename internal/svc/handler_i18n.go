package svc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/DotNetAge/mindx/internal/i18n"
	"golang.org/x/text/language"
)

// --- i18n RPC handlers ---

// handleI18nGet returns the current language information.
func (d *Daemon) handleI18nGet(_ context.Context, _ json.RawMessage) (any, error) {
	tag := i18n.Lang()
	return map[string]any{
		"tag":        tag.String(),
		"name":       i18n.LanguageName(tag),
		"is_chinese": i18n.IsChinese(),
	}, nil
}

// handleI18nSwitch switches the runtime language and persists to config.
type i18nSwitchParams struct {
	Lang string `json:"lang"` // BCP 47 tag: "zh", "en", "zh-TW"
}

func (d *Daemon) handleI18nSwitch(_ context.Context, params json.RawMessage) (any, error) {
	var p i18nSwitchParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}

	if p.Lang == "" {
		return nil, errors.New(i18n.T("rpc.i18n.error.lang.required"))
	}

	// Validate that the target language is supported.
	supported := false
	for _, t := range i18n.SupportedLanguages() {
		if t.String() == p.Lang || baseTag(t.String()) == baseTag(p.Lang) {
			supported = true
			break
		}
	}
	if !supported {
		supportedTags := i18n.SupportedLanguages()
		names := make([]string, len(supportedTags))
		for i, t := range supportedTags {
			names[i] = t.String()
		}
		return map[string]any{
			"supported": names,
		}, errors.New(i18n.T("rpc.i18n.error.unsupported"))
	}

	// Switch runtime translations.
	if err := i18n.SwitchLanguage(p.Lang); err != nil {
		return nil, err
	}

	// Persist to config if available.
	if d.app != nil && d.app.Config() != nil {
		cfg := d.app.Config()
		cfg.Language = p.Lang
		if err := cfg.Save(); err != nil {
			return map[string]any{
				"tag":     i18n.Lang().String(),
				"name":    i18n.LanguageName(i18n.Lang()),
				"warning": i18n.T("rpc.i18n.warning.config.save.failed"),
			}, nil
		}
	}

	return map[string]any{
		"tag":  i18n.Lang().String(),
		"name": i18n.LanguageName(i18n.Lang()),
	}, nil
}

// handleI18nList returns all supported languages with their metadata.
func (d *Daemon) handleI18nList(_ context.Context, _ json.RawMessage) (any, error) {
	languages := i18n.SupportedLanguages()
	list := make([]map[string]any, 0, len(languages))
	for _, tag := range languages {
		list = append(list, map[string]any{
			"tag":  tag.String(),
			"name": i18n.LanguageName(tag),
		})
	}
	current := i18n.Lang()
	return map[string]any{
		"languages": list,
		"current":   current.String(),
	}, nil
}

// baseTag extracts the base language from a BCP 47 tag string (e.g., "zh-TW" -> "zh").
func baseTag(tag string) string {
	for i, r := range tag {
		if r == '-' {
			return tag[:i]
		}
	}
	return tag
}

// _ is a compile-time check that language.Tag satisfies fmt.Stringer.
var _ fmt.Stringer = language.Tag{}
