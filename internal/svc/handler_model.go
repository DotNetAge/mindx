package svc

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"

	goharnessconfig "github.com/DotNetAge/goharness/config"
	"github.com/DotNetAge/mindx/internal/core"
	"github.com/DotNetAge/mindx/pkg/rpc"
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

func (d *Daemon) handleModelGet(_ context.Context, params json.RawMessage) (any, error) {
	var p rpc.ModelGetParams
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

	cfg := models.Get(modelLookupKey(p.Name, p.Provider))
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

	key := modelLookupKey(p.Name, p.Provider)

	cfg := models.Get(key)
	if cfg == nil {
		return nil, fmt.Errorf("model %q not found", p.Name)
	}

	// 参照字段改存组合串（Provider/Name），保证跨供应商同名模型也能被精确记忆与解析。
	d.app.Config().DefaultModel = key
	d.app.Config().LastModel = key
	if p.Provider != "" {
		d.app.Config().DefaultProvider = p.Provider
	}
	if err := d.app.Config().Save(); err != nil {
		return nil, fmt.Errorf("failed to save config: %w", err)
	}

	// 清空 Runtime 缓存：已缓存的 Runtime 持有旧模型的 LLMClient 与配置，
	// 若不清空，切换后仍会用旧模型发起调用（如从 ollama 切到 deepseek 后仍走 ollama）。
	d.app.InvalidateRuntimeCache()

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

	d.app.SetProviderConfigs(allProviders)
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

	d.app.SetProviderConfigs(filtered)

	return map[string]any{
		"name":    p.Name,
		"message": fmt.Sprintf("Provider %q deleted", p.Name),
	}, nil
}

// ollamaBaseURL 从 OpenAI 兼容地址（如 http://localhost:11434/v1）中提取
// scheme://host，用于调用 Ollama 原生 API（/api/tags、/api/show）。
func ollamaBaseURL(rawURL string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}
	u.Path = ""
	u.RawPath = ""
	return strings.TrimRight(u.String(), "/"), nil
}

func (d *Daemon) handleProviderFetchOllamaModels(_ context.Context, params json.RawMessage) (any, error) {
	var p rpc.FetchOllamaModelsParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if p.BaseURL == "" {
		return nil, fmt.Errorf("base_url is required")
	}

	base, err := ollamaBaseURL(p.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("无效的 base_url: %w", err)
	}

	// 调用 Ollama /api/tags 获取已安装的模型列表
	resp, err := http.Get(base + "/api/tags")
	if err != nil {
		return nil, fmt.Errorf("无法连接 Ollama 服务: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取 Ollama 响应失败: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Ollama 返回错误 (HTTP %d): %s", resp.StatusCode, string(body))
	}

	var ollamaResp struct {
		Models []rpc.OllamaModelInfo `json:"models"`
	}
	if err := json.Unmarshal(body, &ollamaResp); err != nil {
		return nil, fmt.Errorf("解析 Ollama 响应失败: %w", err)
	}

	if ollamaResp.Models == nil {
		ollamaResp.Models = []rpc.OllamaModelInfo{}
	}

	return map[string]any{
		"models": ollamaResp.Models,
		"total":  len(ollamaResp.Models),
	}, nil
}

func (d *Daemon) handleProviderFetchOllamaModelDetail(_ context.Context, params json.RawMessage) (any, error) {
	var p rpc.FetchOllamaModelDetailParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if p.BaseURL == "" || p.ModelName == "" {
		return nil, fmt.Errorf("base_url and model_name are required")
	}

	base, err := ollamaBaseURL(p.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("无效的 base_url: %w", err)
	}

	// 调用 Ollama /api/show 获取模型详细信息
	apiURL := base + "/api/show"
	reqBody, _ := json.Marshal(map[string]string{"name": p.ModelName})
	resp, err := http.Post(apiURL, "application/json", strings.NewReader(string(reqBody)))
	if err != nil {
		return nil, fmt.Errorf("无法连接 Ollama 服务: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取 Ollama 响应失败: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Ollama 返回错误 (HTTP %d): %s", resp.StatusCode, string(body))
	}

	var showResp struct {
		Parameters string         `json:"parameters"`
		Details    map[string]any `json:"details"`
		ModelInfo  map[string]any `json:"model_info"`
	}
	if err := json.Unmarshal(body, &showResp); err != nil {
		return nil, fmt.Errorf("解析 Ollama 响应失败: %w", err)
	}

	detail := rpc.OllamaModelDetail{
		Name: p.ModelName,
	}

	// 从 model_info 中提取 context_length：遍历所有 key 寻找以 .context_length 结尾的。
	// ollama 的 key 形如 "qwen35.context_length" / "qwen3.context_length" / "llama.context_length"，
	// 前缀随架构变化，用 strings.HasSuffix 匹配后缀最稳妥。
	// 修复：旧代码用 k[len(k)-16:] 取后16字符比较，但 ".context_length" 只有15字符，
	// 导致永远匹配失败，所有 ollama 模型的 context_length 都被解析为 0，前端兜底成 4096。
	var ctxLen float64
	for k, v := range showResp.ModelInfo {
		if strings.HasSuffix(k, ".context_length") {
			if f, ok := v.(float64); ok && f > 0 {
				ctxLen = f
				break
			}
		}
	}
	// fallback: 从 parameters 字段解析 num_ctx
	if ctxLen == 0 {
		parseOllamaParam(showResp.Parameters, "num_ctx", &ctxLen)
	}
	detail.ContextLength = int64(ctxLen)

	// 提取 details 中的信息
	if showResp.Details != nil {
		if f, ok := showResp.Details["family"].(string); ok {
			detail.ModelFamily = f
		}
		if p, ok := showResp.Details["parameter_size"].(string); ok {
			detail.ParameterSize = p
		}
		if q, ok := showResp.Details["quantization_level"].(string); ok {
			detail.Quantization = q
		}
	}

	// 从 model_info 提取 parameter_count
	if v, ok := showResp.ModelInfo["general.parameter_count"]; ok {
		if f, ok := v.(float64); ok {
			detail.ParameterCount = int64(f)
		}
	}

	return detail, nil
}

// parseOllamaParam 从 Ollama parameters 字符串中解析指定 key 的数值。
func parseOllamaParam(params, key string, out any) {
	for _, line := range strings.Split(params, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, key+" ") || strings.HasPrefix(line, key+"\t") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				switch v := out.(type) {
				case *float64:
					if f, err := strconv.ParseFloat(parts[1], 64); err == nil {
						*v = f
					}
				}
			}
			return
		}
	}
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
	if models.GetRaw(modelLookupKey(p.Name, p.Provider)) != nil {
		return nil, fmt.Errorf("model %q already exists", p.Name)
	}

	newCfg := &goharnessconfig.ModelConfig{
		Name:           p.Name,
		Title:          p.Title,
		Description:    p.Description,
		Provider:       p.Provider,
		BaseURL:        p.BaseURL,
		APIKey:         p.APIKey,
		AuthToken:      p.AuthToken,
		ContextLength:  p.ContextLength,
		IsLocal:        p.IsLocal,
		FuncCalling:    p.FuncCalling,
		Structuring:    p.Structuring,
		WebSearching:   p.WebSearching,
		Visioning:      p.Visioning,
		PrefixCon:      p.PrefixCon,
		ContextCache:   p.ContextCache,
		Temperature:    p.Temperature,
		Enabled:        p.Enabled,
		MaxTurns:       p.MaxTurns,
		RequestTimeout: p.RequestTimeout,
		CostPer1MIn:    p.CostPer1MIn,
		CostPer1MOut:   p.CostPer1MOut,
	}

	if err := models.Save(newCfg); err != nil {
		return nil, fmt.Errorf("failed to save model: %w", err)
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
	raw := models.GetRaw(modelLookupKey(p.Name, p.Provider))
	if raw == nil {
		return nil, fmt.Errorf("model %q not found", p.Name)
	}

	// Provider 属于模型的身份标识（组合键 Provider/Name 的组成部分），不可在 update 中变更。
	// 上面按该 provider 寻址：若调用方传入的 provider 与模型当前 provider 不一致，组合键查询
	// 必然 miss 返回 not found，因此 provider 天然不可变；要迁移模型请 delete 后重建。
	updated := *raw
	if p.Title != "" {
		updated.Title = p.Title
	}
	if p.Description != "" {
		updated.Description = p.Description
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
	if p.Visioning != nil {
		updated.Visioning = *p.Visioning
	}
	if p.PrefixCon != nil {
		updated.PrefixCon = *p.PrefixCon
	}
	if p.ContextCache != nil {
		updated.ContextCache = *p.ContextCache
	}
	if p.Temperature != nil {
		updated.Temperature = *p.Temperature
	}
	if p.Enabled != nil {
		updated.Enabled = *p.Enabled
	}
	if p.MaxTurns != nil {
		updated.MaxTurns = *p.MaxTurns
	}
	if p.RequestTimeout != nil {
		updated.RequestTimeout = *p.RequestTimeout
	}
	if p.CostPer1MIn != nil {
		updated.CostPer1MIn = *p.CostPer1MIn
	}
	if p.CostPer1MOut != nil {
		updated.CostPer1MOut = *p.CostPer1MOut
	}

	if err := models.Save(&updated); err != nil {
		return nil, fmt.Errorf("failed to save model: %w", err)
	}
	// Provider 不可变，组合键不变，无需清理旧键。

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
	key := modelLookupKey(p.Name, p.Provider)
	if models.GetRaw(key) == nil {
		return nil, fmt.Errorf("model %q not found", p.Name)
	}

	// 必须先从内存注册表删除再持久化，否则 model.list 仍会返回该模型，
	// 前端下一次刷新立刻"复活"，看似删除按钮无反应。
	// ModelRegistry.Delete 内部已经做了：1) 内存 map 删除 2) 全量回写 YAML。
	// 旧实现的 deleteModelFromFile 只动文件不动内存，正是该 bug 的根因。
	if err := models.Delete(key); err != nil {
		return nil, fmt.Errorf("failed to delete model: %w", err)
	}

	return map[string]any{
		"name":    p.Name,
		"message": fmt.Sprintf("Model %q deleted", p.Name),
	}, nil
}

// --- Helpers ---

// modelLookupKey 构造模型在注册表中的组合寻址键：Provider + "/" + Name；
// Provider 为空时退化为仅 Name。与 goharness config 中 modelKey 的格式保持一致，
// 用于在跨供应商存在同名模型时精确寻址。
func modelLookupKey(name, provider string) string {
	if provider == "" {
		return name
	}
	return provider + "/" + name
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
