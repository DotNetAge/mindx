package svc

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	goreactconfig "github.com/DotNetAge/goreact/config"
	"github.com/DotNetAge/mindx/internal/core"
	"gopkg.in/yaml.v3"
)

func (d *Daemon) handleModelList(_ context.Context, _ json.RawMessage) (any, error) {
	models := d.app.Models()
	if models == nil {
		return []goreactconfig.ModelConfig{}, nil
	}
	list := models.List()
	if list == nil {
		return []goreactconfig.ModelConfig{}, nil
	}
	return list, nil
}

func (d *Daemon) handleProviderList(_ context.Context, _ json.RawMessage) (any, error) {
	providers := d.app.ProviderConfigs()
	if providers == nil {
		return []any{}, nil
	}
	result := make([]any, 0, len(providers))
	for _, p := range providers {
		result = append(result, map[string]any{
			"name":     p.Name,
			"title":    p.Title,
			"base_url": p.BaseURL,
			"api_key":  len(p.APIKey) > 0,
			"is_local": p.IsLocal,
		})
	}
	return result, nil
}

type modelGetParams struct {
	Name string `json:"name"`
}

func (d *Daemon) handleModelGet(_ context.Context, params json.RawMessage) (any, error) {
	var p modelGetParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if p.Name == "" {
		return nil, fmt.Errorf("name is required")
	}

	models := d.app.Models()
	if models == nil {
		return nil, fmt.Errorf("model registry not available")
	}

	cfg := models.Get(p.Name)
	if cfg == nil {
		return nil, fmt.Errorf("model %q not found", p.Name)
	}

	return cfg, nil
}

type modelSwitchParams struct {
	Name     string `json:"name"`
	Provider string `json:"provider,omitempty"`
}

func (d *Daemon) handleModelSwitch(_ context.Context, params json.RawMessage) (any, error) {
	var p modelSwitchParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if p.Name == "" {
		return nil, fmt.Errorf("name is required")
	}

	models := d.app.Models()
	if models == nil {
		return nil, fmt.Errorf("model registry not available")
	}

	cfg := models.Get(p.Name)
	if cfg == nil {
		return nil, fmt.Errorf("model %q not found", p.Name)
	}

	d.app.Config().DefaultModel = p.Name
	if p.Provider != "" {
		d.app.Config().DefaultProvider = p.Provider
	}
	if err := d.app.Config().Save(); err != nil {
		return nil, fmt.Errorf("failed to save config: %w", err)
	}

	return map[string]any{
		"name":     p.Name,
		"provider": cfg.Provider,
		"message":  fmt.Sprintf("Switched to model %q", p.Name),
	}, nil
}

// --- Provider CRUD ---

type providerCreateParams struct {
	Name      string `json:"name"`
	Title     string `json:"title"`
	BaseURL   string `json:"base_url"`
	APIKey    string `json:"api_key"`
	AuthToken string `json:"auth_token,omitempty"`
	IsLocal   bool   `json:"is_local,omitempty"`
}

func (d *Daemon) handleProviderCreate(_ context.Context, params json.RawMessage) (any, error) {
	var p providerCreateParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if p.Name == "" {
		return nil, fmt.Errorf("provider name is required")
	}

	existing := d.app.ProviderConfigs()
	for _, ep := range existing {
		if ep.Name == p.Name {
			return nil, fmt.Errorf("provider %q already exists", p.Name)
		}
	}

	newProvider := &goreactconfig.ProviderConfig{
		Name:      p.Name,
		Title:     p.Title,
		BaseURL:   p.BaseURL,
		APIKey:    p.APIKey,
		AuthToken: p.AuthToken,
		IsLocal:   p.IsLocal,
	}

	allProviders := append(existing, newProvider)
	if err := core.SaveProvidersFile(d.app.Settings().ProvidersFile(), allProviders); err != nil {
		return nil, fmt.Errorf("failed to save providers: %w", err)
	}

	d.app.Models().RegisterProvider(p.Name, newProvider)

	return map[string]any{
		"name":     newProvider.Name,
		"title":    newProvider.Title,
		"base_url": newProvider.BaseURL,
		"message":  fmt.Sprintf("Provider %q created", p.Name),
	}, nil
}

type providerUpdateParams struct {
	Name      string `json:"name"`
	Title     string `json:"title,omitempty"`
	BaseURL   string `json:"base_url,omitempty"`
	APIKey    string `json:"api_key,omitempty"`
	AuthToken string `json:"auth_token,omitempty"`
	IsLocal   *bool  `json:"is_local,omitempty"`
}

func (d *Daemon) handleProviderUpdate(_ context.Context, params json.RawMessage) (any, error) {
	var p providerUpdateParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if p.Name == "" {
		return nil, fmt.Errorf("provider name is required")
	}

	existing := d.app.ProviderConfigs()
	found := false
	for i, ep := range existing {
		if ep.Name == p.Name {
			found = true
			if p.Title != "" {
				existing[i].Title = p.Title
			}
			if p.BaseURL != "" {
				existing[i].BaseURL = p.BaseURL
			}
			if paramsContainsKey(params, "api_key") {
				existing[i].APIKey = p.APIKey
			}
			if paramsContainsKey(params, "auth_token") {
				existing[i].AuthToken = p.AuthToken
			}
			if p.IsLocal != nil {
				existing[i].IsLocal = *p.IsLocal
			}
			break
		}
	}

	if !found {
		return nil, fmt.Errorf("provider %q not found", p.Name)
	}

	if err := core.SaveProvidersFile(d.app.Settings().ProvidersFile(), existing); err != nil {
		return nil, fmt.Errorf("failed to save providers: %w", err)
	}

	idx := providerIndex(existing, p.Name)
	if idx >= 0 {
		d.app.Models().RegisterProvider(p.Name, existing[idx])
	}

	return map[string]any{
		"name":     p.Name,
		"title":    existing[idx].Title,
		"base_url": existing[idx].BaseURL,
		"message":  fmt.Sprintf("Provider %q updated", p.Name),
	}, nil
}

type providerDeleteParams struct {
	Name string `json:"name"`
}

func (d *Daemon) handleProviderDelete(_ context.Context, params json.RawMessage) (any, error) {
	var p providerDeleteParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if p.Name == "" {
		return nil, fmt.Errorf("provider name is required")
	}

	existing := d.app.ProviderConfigs()
	filtered := make([]*goreactconfig.ProviderConfig, 0, len(existing))
	for _, ep := range existing {
		if ep.Name != p.Name {
			filtered = append(filtered, ep)
		}
	}

	if len(filtered) == len(existing) {
		return nil, fmt.Errorf("provider %q not found", p.Name)
	}

	if err := core.SaveProvidersFile(d.app.Settings().ProvidersFile(), filtered); err != nil {
		return nil, fmt.Errorf("failed to save providers: %w", err)
	}

	return map[string]any{
		"name":    p.Name,
		"message": fmt.Sprintf("Provider %q deleted", p.Name),
	}, nil
}

// --- Model CRUD ---

type modelCreateParams struct {
	Name              string  `json:"name"`
	Title             string  `json:"title"`
	Description       string  `json:"description,omitempty"`
	Provider          string  `json:"provider"`
	BaseURL           string  `json:"base_url,omitempty"`
	APIKey            string  `json:"api_key,omitempty"`
	AuthToken         string  `json:"auth_token,omitempty"`
	MaxTokens         int64   `json:"max_tokens,omitempty"`
	ContextLength     int64   `json:"context_length,omitempty"`
	IsLocal           bool    `json:"is_local,omitempty"`
	FuncCalling       bool    `json:"func_calling,omitempty"`
	Structuring       bool    `json:"structuring,omitempty"`
	WebSearching      bool    `json:"web_searching,omitempty"`
	PrefixCon         bool    `json:"prefix_con,omitempty"`
	ContextCache      bool    `json:"context_cache,omitempty"`
	TopP              float64 `json:"top_p,omitempty"`
	TopK              float64 `json:"top_k,omitempty"`
	Temperature       float64 `json:"temperature,omitempty"`
	RepetitionPenalty float64 `json:"repetition_penalty,omitempty"`
	FrequencyPenalty  float64 `json:"frequency_penalty,omitempty"`
	Enabled           bool    `json:"enabled,omitempty"`
	MaxTurns          int     `json:"max_turns,omitempty"`
	CostPer1MIn       float64 `json:"cost_per_1m_in,omitempty"`
	CostPer1MOut      float64 `json:"cost_per_1m_out,omitempty"`
}

func (d *Daemon) handleModelCreate(_ context.Context, params json.RawMessage) (any, error) {
	var p modelCreateParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if p.Name == "" {
		return nil, fmt.Errorf("model name is required")
	}

	models := d.app.Models()
	if models.GetRaw(p.Name) != nil {
		return nil, fmt.Errorf("model %q already exists", p.Name)
	}

	newCfg := &goreactconfig.ModelConfig{
		Name:              p.Name,
		Title:             p.Title,
		Description:       p.Description,
		Provider:          p.Provider,
		BaseURL:           p.BaseURL,
		APIKey:            p.APIKey,
		AuthToken:         p.AuthToken,
		MaxTokens:         p.MaxTokens,
		ContextLength:     p.ContextLength,
		IsLocal:           p.IsLocal,
		FuncCalling:       p.FuncCalling,
		Structuring:       p.Structuring,
		WebSearching:      p.WebSearching,
		PrefixCon:         p.PrefixCon,
		ContextCache:      p.ContextCache,
		TopP:              p.TopP,
		TopK:              p.TopK,
		Temperature:       p.Temperature,
		RepetitionPenalty: p.RepetitionPenalty,
		FrequencyPenalty:  p.FrequencyPenalty,
		Enabled:           p.Enabled,
		MaxTurns:          p.MaxTurns,
	}

	if err := models.Save(newCfg); err != nil {
		return nil, fmt.Errorf("failed to save model: %w", err)
	}

	if p.CostPer1MIn > 0 || p.CostPer1MOut > 0 {
		d.app.Costs().Set(p.Name, core.ModelCost{CostPer1MIn: p.CostPer1MIn, CostPer1MOut: p.CostPer1MOut})
	}

	return map[string]any{
		"name":    newCfg.Name,
		"title":   newCfg.Title,
		"message": fmt.Sprintf("Model %q created", p.Name),
	}, nil
}

type modelUpdateParams struct {
	Name              string   `json:"name"`
	Title             string   `json:"title,omitempty"`
	Description       string   `json:"description,omitempty"`
	Provider          string   `json:"provider,omitempty"`
	BaseURL           string   `json:"base_url,omitempty"`
	APIKey            string   `json:"api_key,omitempty"`
	AuthToken         string   `json:"auth_token,omitempty"`
	MaxTokens         *int64   `json:"max_tokens,omitempty"`
	ContextLength     *int64   `json:"context_length,omitempty"`
	IsLocal           *bool    `json:"is_local,omitempty"`
	FuncCalling       *bool    `json:"func_calling,omitempty"`
	Structuring       *bool    `json:"structuring,omitempty"`
	WebSearching      *bool    `json:"web_searching,omitempty"`
	PrefixCon         *bool    `json:"prefix_con,omitempty"`
	ContextCache      *bool    `json:"context_cache,omitempty"`
	TopP              *float64 `json:"top_p,omitempty"`
	TopK              *float64 `json:"top_k,omitempty"`
	Temperature       *float64 `json:"temperature,omitempty"`
	RepetitionPenalty *float64 `json:"repetition_penalty,omitempty"`
	FrequencyPenalty  *float64 `json:"frequency_penalty,omitempty"`
	Enabled           *bool    `json:"enabled,omitempty"`
	MaxTurns          *int     `json:"max_turns,omitempty"`
	CostPer1MIn       *float64 `json:"cost_per_1m_in,omitempty"`
	CostPer1MOut      *float64 `json:"cost_per_1m_out,omitempty"`
}

func (d *Daemon) handleModelUpdate(_ context.Context, params json.RawMessage) (any, error) {
	var p modelUpdateParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if p.Name == "" {
		return nil, fmt.Errorf("model name is required")
	}

	models := d.app.Models()
	raw := models.GetRaw(p.Name)
	if raw == nil {
		return nil, fmt.Errorf("model %q not found", p.Name)
	}

	updated := *raw
	if p.Title != "" { updated.Title = p.Title }
	if p.Description != "" { updated.Description = p.Description }
	if p.Provider != "" { updated.Provider = p.Provider }
	if p.BaseURL != "" { updated.BaseURL = p.BaseURL }
	if paramsContainsKey(params, "api_key") { updated.APIKey = p.APIKey }
	if paramsContainsKey(params, "auth_token") { updated.AuthToken = p.AuthToken }
	if p.MaxTokens != nil { updated.MaxTokens = *p.MaxTokens }
	if p.ContextLength != nil { updated.ContextLength = *p.ContextLength }
	if p.IsLocal != nil { updated.IsLocal = *p.IsLocal }
	if p.FuncCalling != nil { updated.FuncCalling = *p.FuncCalling }
	if p.Structuring != nil { updated.Structuring = *p.Structuring }
	if p.WebSearching != nil { updated.WebSearching = *p.WebSearching }
	if p.PrefixCon != nil { updated.PrefixCon = *p.PrefixCon }
	if p.ContextCache != nil { updated.ContextCache = *p.ContextCache }
	if p.TopP != nil { updated.TopP = *p.TopP }
	if p.TopK != nil { updated.TopK = *p.TopK }
	if p.Temperature != nil { updated.Temperature = *p.Temperature }
	if p.RepetitionPenalty != nil { updated.RepetitionPenalty = *p.RepetitionPenalty }
	if p.FrequencyPenalty != nil { updated.FrequencyPenalty = *p.FrequencyPenalty }
	if p.Enabled != nil { updated.Enabled = *p.Enabled }
	if p.MaxTurns != nil { updated.MaxTurns = *p.MaxTurns }

	if err := models.Save(&updated); err != nil {
		return nil, fmt.Errorf("failed to save model: %w", err)
	}

	if p.CostPer1MIn != nil || p.CostPer1MOut != nil {
		mc := core.ModelCost{}
		if p.CostPer1MIn != nil { mc.CostPer1MIn = *p.CostPer1MIn }
		if p.CostPer1MOut != nil { mc.CostPer1MOut = *p.CostPer1MOut }
		d.app.Costs().Set(p.Name, mc)
	}

	return map[string]any{
		"name":    updated.Name,
		"title":   updated.Title,
		"message": fmt.Sprintf("Model %q updated", p.Name),
	}, nil
}

func (d *Daemon) handleModelDelete(_ context.Context, params json.RawMessage) (any, error) {
	var p modelGetParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if p.Name == "" {
		return nil, fmt.Errorf("model name is required")
	}

	models := d.app.Models()
	if models.GetRaw(p.Name) == nil {
		return nil, fmt.Errorf("model %q not found", p.Name)
	}

	if err := deleteModelFromFile(models, d.app.Settings().ModelsFile(), p.Name); err != nil {
		return nil, fmt.Errorf("failed to delete model: %w", err)
	}

	return map[string]any{
		"name":    p.Name,
		"message": fmt.Sprintf("Model %q deleted", p.Name),
	}, nil
}

// --- Helpers ---

func deleteModelFromFile(models interface{ GetRaw(string) *goreactconfig.ModelConfig }, settingPath string, name string) error {
	data, err := os.ReadFile(settingPath)
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}

	var wrapper struct {
		Models    []goreactconfig.ModelConfig    `yaml:"models"`
		Providers []goreactconfig.ProviderConfig `yaml:"providers,omitempty"`
	}
	if err := yaml.Unmarshal(data, &wrapper); err != nil {
		return fmt.Errorf("parse yaml: %w", err)
	}

	filtered := make([]goreactconfig.ModelConfig, 0, len(wrapper.Models))
	for _, m := range wrapper.Models {
		if m.Name != name {
			filtered = append(filtered, m)
		}
	}
	if len(filtered) == len(wrapper.Models) {
		return fmt.Errorf("model %q not found in file", name)
	}

	wrapper.Models = filtered

	outData, err := yaml.Marshal(wrapper)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	if err := os.WriteFile(settingPath, outData, 0644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	return nil
}

func paramsContainsKey(raw json.RawMessage, key string) bool {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return false
	}
	_, ok := m[key]
	return ok
}

func providerIndex(providers []*goreactconfig.ProviderConfig, name string) int {
	for i, p := range providers {
		if p.Name == name {
			return i
		}
	}
	return -1
}
