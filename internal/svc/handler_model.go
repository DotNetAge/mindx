package svc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	goharnessconfig "github.com/DotNetAge/goharness/config"
	"github.com/DotNetAge/mindx/internal/core"
	"github.com/DotNetAge/mindx/pkg/rpc"
)

// upstreamErrorText 把上游模型服务（OpenRouter/Kimi/MiniMax/Ollama 等）的错误响应
// 压成人话：优先取 JSON 的 error.message（OpenAI 风格）或 error 字符串（Ollama 风格），
// 避免把原始 JSON 透出到界面（UI 文案军规）；解析不出时给人话回落，不透原文。
func upstreamErrorText(provider string, status int, body []byte) error {
	var probe struct {
		Error json.RawMessage `json:"error"`
	}
	if err := json.Unmarshal(body, &probe); err == nil && len(probe.Error) > 0 {
		var withMsg struct {
			Message string `json:"message"`
		}
		if json.Unmarshal(probe.Error, &withMsg) == nil && withMsg.Message != "" {
			return fmt.Errorf("%s", withMsg.Message)
		}
		var asText string
		if json.Unmarshal(probe.Error, &asText) == nil && asText != "" {
			return fmt.Errorf("%s", asText)
		}
	}
	return fmt.Errorf("调用%s失败（HTTP %d），请检查服务配置或稍后重试", provider, status)
}

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

	// 模型在 Runtime 构建时按 provider 继承连接参数，新增 provider 后
	// 必须失效缓存，确保后续 Runtime 解析到最新的 provider 配置。
	d.app.InvalidateRuntimeCache()

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

	// 已缓存的 Runtime 持有创建时按旧 provider 配置（含凭证引用解析结果）
	// 生成的快照，凭证/BaseURL 变更后必须失效缓存，否则后续请求仍用旧
	// 快照发起调用，表现为「保存了正确的 Key 却一直 401 API Key not exists」。
	d.app.InvalidateRuntimeCache()

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

	// 已缓存的 Runtime 可能引用被删 provider 的连接参数，失效缓存避免悬空引用。
	d.app.InvalidateRuntimeCache()

	return map[string]any{
		"name":    p.Name,
		"message": fmt.Sprintf("Provider %q deleted", p.Name),
	}, nil
}

// --- Provider 连通性验证 ---

// handleProviderVerify 处理 provider.verify：用该供应商下价格最低的模型发一次
// 最小对话请求，验证 API Key 是否真实可用。验证结果以 ok/message 数据形式返回
// （不走 RPC error 通道），前端把 message 原样讲给用户听——message 必须解释
// 状态码含义并给出下一步动作，禁止只回一个裸状态码（UI 文案军规）。
func (d *Daemon) handleProviderVerify(_ context.Context, params json.RawMessage) (any, error) {
	var p rpc.ProviderVerifyParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}

	provider, err := resolveProviderConfig(d, p.Name)
	if err != nil {
		return nil, err
	}

	// 与 provider.list 的「已配置」判定同一口径：只认凭据库解析结果。
	// 不用 resolveProviderAPIKey（其 YAML 回退会把 providers.yml 的引用名
	// 如 MOONSHOT_API_KEY 当成 Key，让未配置供应商被误判为已配置）。
	credStore := core.NewCredentialStore(d.app.Settings().UserPreferences())
	apiKey := core.ResolveAPIKey(credStore, provider.Name)
	if apiKey == "" {
		return map[string]any{
			"ok":      false,
			"message": "该供应商还没有配置 API Key，先填写 Key 再验证。",
		}, nil
	}

	base := ensureV1Suffix(provider.BaseURL)
	if base == "" {
		return map[string]any{
			"ok":      false,
			"message": "该供应商还没有配置接口地址，先补全接口地址再验证。",
		}, nil
	}

	candidates := make([]*goharnessconfig.ModelConfig, 0, len(d.app.Models().List()))
	for _, m := range d.app.Models().List() {
		if m != nil && m.Provider == p.Name && m.Name != "" {
			candidates = append(candidates, m)
		}
	}
	if len(candidates) == 0 {
		return map[string]any{
			"ok":      false,
			"message": "该供应商下还没有模型，先添加一个模型再验证。",
		}, nil
	}
	// 探针优先选未停用的模型；全部停用时退回全量候选（验证的是 Key，不是模型可用性）
	probe := cheapestModel(candidates, true)
	if probe == nil {
		probe = cheapestModel(candidates, false)
	}

	// 最小对话请求：max_tokens=16 防止个别供应商拒绝过小的取值，成本可忽略
	payload, _ := json.Marshal(map[string]any{
		"model":      probe.Name,
		"messages":   []map[string]string{{"role": "user", "content": "hi"}},
		"max_tokens": 16,
		"stream":     false,
	})
	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest(http.MethodPost, base+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("构造验证请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := client.Do(req)
	if err != nil {
		return map[string]any{
			"ok":      false,
			"message": "连不上供应商服务：请检查电脑网络，或核对供应商的接口地址是否正确。",
			"model":   probe.Name,
		}, nil
	}
	defer func() { _ = resp.Body.Close() }()

	// 响应体只用于解读错误，LimitReader 防故障端点耗尽内存
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("读取验证响应失败: %w", err)
	}

	if resp.StatusCode == http.StatusOK {
		return map[string]any{
			"ok":      true,
			"message": fmt.Sprintf("验证通过：已用模型 %s 实际调用成功，这个 Key 可以正常使用。", probe.Name),
			"model":   probe.Name,
		}, nil
	}
	return map[string]any{
		"ok":      false,
		"message": verifyFailureText(p.Name, resp.StatusCode, body),
		"model":   probe.Name,
		"status":  resp.StatusCode,
	}, nil
}

// cheapestModel 从候选中选验证探针：按输入+输出单价之和最小者；未标价的模型
// 视为最贵（排最后），把验证的真实计费压到最低。全部未标价时保持清单原序取第一个。
func cheapestModel(models []*goharnessconfig.ModelConfig, enabledOnly bool) *goharnessconfig.ModelConfig {
	var best *goharnessconfig.ModelConfig
	bestCost := math.MaxFloat64
	for _, m := range models {
		if enabledOnly && !m.Enabled {
			continue
		}
		cost := m.CostPer1MIn + m.CostPer1MOut
		if cost < bestCost {
			best, bestCost = m, cost
		}
	}
	return best
}

// verifyFailureText 把验证失败的 HTTP 状态码翻译成人话：先说明这个码代表什么、
// 该做什么，再附供应商返回的原始说明（若有），绝不只回一个裸状态码。
func verifyFailureText(provider string, status int, body []byte) string {
	var detail string
	var probe struct {
		Error json.RawMessage `json:"error"`
	}
	if err := json.Unmarshal(body, &probe); err == nil && len(probe.Error) > 0 {
		var withMsg struct {
			Message string `json:"message"`
		}
		if json.Unmarshal(probe.Error, &withMsg) == nil && withMsg.Message != "" {
			detail = withMsg.Message
		} else {
			var asText string
			if json.Unmarshal(probe.Error, &asText) == nil && asText != "" {
				detail = asText
			}
		}
	}

	var meaning string
	switch status {
	case http.StatusBadRequest:
		meaning = "代码 400 表示请求本身不被接受：一般是接口地址写错了，或这个模型不支持对话调用"
	case http.StatusUnauthorized:
		meaning = "代码 401 表示身份验证失败：API Key 无效、已过期或已被删除，请重新填写正确的 Key"
	case http.StatusPaymentRequired:
		meaning = "代码 402 表示账户余额不足：请到供应商平台充值后再试"
	case http.StatusForbidden:
		meaning = "代码 403 表示 Key 有效但没有权限：你的账号可能还没开通这个模型的调用权限"
	case http.StatusNotFound:
		meaning = "代码 404 表示找不到这个模型或接口：请核对模型名与接口地址是否配套"
	case http.StatusRequestTimeout:
		meaning = "代码 408 表示请求超时：供应商响应太慢，请稍后再试"
	case http.StatusTooManyRequests:
		meaning = "代码 429 表示请求太频繁或额度已用尽：请稍后再试，或检查账户的用量限制"
	case http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		meaning = fmt.Sprintf("代码 %d 表示供应商服务暂时故障：不是你的配置问题，请稍后再试", status)
	default:
		meaning = fmt.Sprintf("代码 %d 表示验证被供应商拒绝，具体原因见下方说明", status)
	}

	text := fmt.Sprintf("%s 验证未通过（%s）。", provider, meaning)
	if detail != "" {
		text += " 供应商返回的说明：" + detail
	}
	return text
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
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取 Ollama 响应失败: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, upstreamErrorText("Ollama", resp.StatusCode, body)
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
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取 Ollama 响应失败: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, upstreamErrorText("Ollama", resp.StatusCode, body)
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
		Name:             p.Name,
		Title:            p.Title,
		Description:      p.Description,
		Provider:         p.Provider,
		BaseURL:          p.BaseURL,
		APIKey:           p.APIKey,
		AuthToken:        p.AuthToken,
		ContextLength:    p.ContextLength,
		IsLocal:          p.IsLocal,
		FuncCalling:      p.FuncCalling,
		Structuring:      p.Structuring,
		WebSearching:     p.WebSearching,
		Visioning:        p.Visioning,
		PrefixCon:        p.PrefixCon,
		ContextCache:     p.ContextCache,
		Temperature:      p.Temperature,
		Enabled:          p.Enabled,
		MaxTurns:         p.MaxTurns,
		RequestTimeout:   p.RequestTimeout,
		CostPer1MIn:      p.CostPer1MIn,
		CostPer1MOut:     p.CostPer1MOut,
		CostPer1MInCache: p.CostPer1MInCache,
	}

	if err := models.Save(newCfg); err != nil {
		return nil, fmt.Errorf("failed to save model: %w", err)
	}

	// 新模型可能成为当前模型的继承来源（同 provider 继承连接参数），
	// 失效缓存确保后续 Runtime 构建能拿到最新注册表。
	d.app.InvalidateRuntimeCache()

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
	if p.CostPer1MInCache != nil {
		updated.CostPer1MInCache = *p.CostPer1MInCache
	}

	if err := models.Save(&updated); err != nil {
		return nil, fmt.Errorf("failed to save model: %w", err)
	}
	// Provider 不可变，组合键不变，无需清理旧键。

	// 已缓存的 Runtime 持有创建时解析的模型配置快照（含 APIKey/BaseURL），
	// 凭证或连接参数变更后必须失效缓存，否则后续请求仍用旧快照发起调用，
	// 表现为「保存了正确的 Key 却一直 401 API Key not exists」。
	d.app.InvalidateRuntimeCache()

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

	// 已缓存的 Runtime 可能正持有被删模型的配置快照，失效缓存避免悬空引用。
	d.app.InvalidateRuntimeCache()

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
