package svc

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	goharnessconfig "github.com/DotNetAge/goharness/config"
	"github.com/DotNetAge/mindx/internal/core"
	"github.com/DotNetAge/mindx/pkg/rpc"
	"gopkg.in/yaml.v3"
)

func (d *Daemon) handleModelList(_ context.Context, _ json.RawMessage) (any, error) {
	models := d.app.Models()
	if models == nil {
		return []goharnessconfig.ModelConfig{}, nil
	}
	list := models.List()
	if list == nil {
		return []goharnessconfig.ModelConfig{}, nil
	}
	return list, nil
}

func (d *Daemon) handleProviderList(_ context.Context, _ json.RawMessage) (any, error) {
	providers := d.app.ProviderConfigs()
	if providers == nil {
		return []any{}, nil
	}

	credStore := core.NewCredentialStore(d.app.Settings().UserPreferences())

	result := make([]any, 0, len(providers))
	for _, p := range providers {
		configured := false
		resolved := core.ResolveAPIKey(credStore, p.Name)
		configured = resolved != ""

		result = append(result, map[string]any{
			"name":     p.Name,
			"title":    p.Title,
			"base_url": p.BaseURL,
			"api_key":  configured,
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

func (d *Daemon) handleModelSwitch(_ context.Context, params json.RawMessage) (any, error) {
	var p rpc.ModelSwitchParams
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
	d.app.Config().LastModel = p.Name
	if p.Provider != "" {
		d.app.Config().DefaultProvider = p.Provider
	}
	if err := d.app.Config().Save(); err != nil {
		return nil, fmt.Errorf("failed to save config: %w", err)
	}

	// If filewatch was not initialized at startup (no model configured),
	// try to initialize it now that a model is available.
	if d.kbWatch == nil {
		if initErr := d.ensureGraphIndexer(); initErr != nil {
			d.logger.Warn("failed to initialize GraphIndexer/FileWatch after model switch",
				"error", initErr,
			)
		}
	}

	return map[string]any{
		"name":     p.Name,
		"provider": cfg.Provider,
		"message":  fmt.Sprintf("Switched to model %q", p.Name),
	}, nil
}

// --- Provider CRUD ---

func (d *Daemon) handleProviderCreate(_ context.Context, params json.RawMessage) (any, error) {
	var p rpc.ProviderCreateParams
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

	newProvider := &goharnessconfig.ProviderConfig{
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

func (d *Daemon) handleProviderUpdate(_ context.Context, params json.RawMessage) (any, error) {
	var p rpc.ProviderUpdateParams
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
			// 规则4: WebUI设置api_key时，先从环境变量尝试读取实际值，
			// 有值则以provider name为键存CredentialStore；无值则以用户输入为值存CredentialStore。
			// 绝不将原始值明文写入YAML配置文件。
			if paramsContainsKey(params, "api_key") {
				storeAndResolveProviderAPIKey(d, existing[i].Name, p.APIKey)
				existing[i].APIKey = existing[i].Name // YAML中只存引用（provider name）
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

func (d *Daemon) handleProviderDelete(_ context.Context, params json.RawMessage) (any, error) {
	var p rpc.ProviderDeleteParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if p.Name == "" {
		return nil, fmt.Errorf("provider name is required")
	}

	existing := d.app.ProviderConfigs()
	filtered := make([]*goharnessconfig.ProviderConfig, 0, len(existing))
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

func (d *Daemon) handleModelCreate(_ context.Context, params json.RawMessage) (any, error) {
	var p rpc.ModelCreateParams
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

	newCfg := &goharnessconfig.ModelConfig{
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

func (d *Daemon) handleModelUpdate(_ context.Context, params json.RawMessage) (any, error) {
	var p rpc.ModelUpdateParams
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
	if p.Title != "" {
		updated.Title = p.Title
	}
	if p.Description != "" {
		updated.Description = p.Description
	}
	if p.Provider != "" {
		updated.Provider = p.Provider
	}
	if p.BaseURL != "" {
		updated.BaseURL = p.BaseURL
	}
	// 规则4: WebUI设置api_key时，先从环境变量尝试读取实际值，
	// 有值则以model.provider为键存CredentialStore；无值则以用户输入为值存CredentialStore。
	// 绝不将原始值明文写入YAML配置文件。
	if paramsContainsKey(params, "api_key") {
		storeKey := updated.Provider
		if storeKey == "" {
			storeKey = updated.Name
		}
		storeAndResolveAPIKey(d, storeKey, p.APIKey)
		updated.APIKey = storeKey // YAML中只存引用（provider name 或 model name）
	}
	if paramsContainsKey(params, "auth_token") {
		updated.AuthToken = p.AuthToken
	}
	if p.MaxTokens != nil {
		updated.MaxTokens = *p.MaxTokens
	}
	if p.ContextLength != nil {
		updated.ContextLength = *p.ContextLength
	}
	if p.IsLocal != nil {
		updated.IsLocal = *p.IsLocal
	}
	if p.FuncCalling != nil {
		updated.FuncCalling = *p.FuncCalling
	}
	if p.Structuring != nil {
		updated.Structuring = *p.Structuring
	}
	if p.WebSearching != nil {
		updated.WebSearching = *p.WebSearching
	}
	if p.PrefixCon != nil {
		updated.PrefixCon = *p.PrefixCon
	}
	if p.ContextCache != nil {
		updated.ContextCache = *p.ContextCache
	}
	if p.TopP != nil {
		updated.TopP = *p.TopP
	}
	if p.TopK != nil {
		updated.TopK = *p.TopK
	}
	if p.Temperature != nil {
		updated.Temperature = *p.Temperature
	}
	if p.RepetitionPenalty != nil {
		updated.RepetitionPenalty = *p.RepetitionPenalty
	}
	if p.FrequencyPenalty != nil {
		updated.FrequencyPenalty = *p.FrequencyPenalty
	}
	if p.Enabled != nil {
		updated.Enabled = *p.Enabled
	}
	if p.MaxTurns != nil {
		updated.MaxTurns = *p.MaxTurns
	}

	if err := models.Save(&updated); err != nil {
		return nil, fmt.Errorf("failed to save model: %w", err)
	}

	if p.CostPer1MIn != nil || p.CostPer1MOut != nil {
		mc := core.ModelCost{}
		if p.CostPer1MIn != nil {
			mc.CostPer1MIn = *p.CostPer1MIn
		}
		if p.CostPer1MOut != nil {
			mc.CostPer1MOut = *p.CostPer1MOut
		}
		d.app.Costs().Set(p.Name, mc)
	}

	return map[string]any{
		"name":    updated.Name,
		"title":   updated.Title,
		"message": fmt.Sprintf("Model %q updated", p.Name),
	}, nil
}

func (d *Daemon) handleModelDelete(_ context.Context, params json.RawMessage) (any, error) {
	var p rpc.ModelGetParams
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

func deleteModelFromFile(models interface {
	GetRaw(string) *goharnessconfig.ModelConfig
}, settingPath string, name string) error {
	data, err := os.ReadFile(settingPath)
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}

	var wrapper struct {
		Models    []goharnessconfig.ModelConfig    `yaml:"models"`
		Providers []goharnessconfig.ProviderConfig `yaml:"providers,omitempty"`
	}
	if err := yaml.Unmarshal(data, &wrapper); err != nil {
		return fmt.Errorf("parse yaml: %w", err)
	}

	filtered := make([]goharnessconfig.ModelConfig, 0, len(wrapper.Models))
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

func providerIndex(providers []*goharnessconfig.ProviderConfig, name string) int {
	for i, p := range providers {
		if p.Name == name {
			return i
		}
	}
	return -1
}

// storeAndResolveProviderAPIKey 实现规则4：处理 WebUI/Daemon RPC 设置 Provider APIKey 的请求。
// 先以用户输入值为键尝试从环境变量读取实际值，有值则存CredentialStore；
// 无值则直接以用户输入值作为实际值存入CredentialStore（以providerName为键）。
func storeAndResolveProviderAPIKey(d *Daemon, providerName, userInput string) {
	if userInput == "" || providerName == "" {
		return
	}
	credStore := core.NewCredentialStore(d.app.Settings().UserPreferences())
	// 规则4: 先尝试从环境变量中以用户提供的值作为键读取
	var actualValue string
	if v := os.Getenv(userInput); v != "" {
		actualValue = v
	} else {
		actualValue = userInput
	}
	_ = credStore.Set(providerName, actualValue)
}

// storeAndResolveAPIKey 通用版本：以指定storeKey为键将解析后的APIKey存入CredentialStore。
func storeAndResolveAPIKey(d *Daemon, storeKey, userInput string) {
	storeAndResolveProviderAPIKey(d, storeKey, userInput)
}
