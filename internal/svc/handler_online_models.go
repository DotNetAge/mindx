package svc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	goharnessconfig "github.com/DotNetAge/goharness/config"
	"github.com/DotNetAge/mindx/internal/core"
	"github.com/DotNetAge/mindx/pkg/rpc"
)

// resolveProviderConfig 按名称查找供应商配置（非空校验 + 存在性校验），
// 供在线模型浏览的两个 handler 复用。
func resolveProviderConfig(d *Daemon, name string) (*goharnessconfig.ProviderConfig, error) {
	if name == "" {
		return nil, fmt.Errorf("provider 不能为空")
	}
	for _, p := range d.app.ProviderConfigs() {
		if p.Name == name {
			return p, nil
		}
	}
	return nil, fmt.Errorf("供应商 %q 不存在", name)
}

// resolveProviderAPIKey 解析供应商的实际 API Key：
// 凭据库（Keychain/加密文件）优先（规则 4：YAML 中只存引用），
// 回退 YAML 中可能存在的原始值（provider.create 路径未走凭据库）。
func resolveProviderAPIKey(d *Daemon, p *goharnessconfig.ProviderConfig) string {
	credStore := core.NewCredentialStore(d.app.Settings().UserPreferences())
	if v := core.ResolveAPIKey(credStore, p.Name); v != "" {
		return v
	}
	return p.APIKey
}

// ensureV1Suffix 规范化 OpenAI 兼容生态的 base_url：
// 用户配置常省略版本段（如 https://api.siliconflow.cn、https://openrouter.ai/api），
// 而模型列表端点位于 /v1/models，缺 /v1 会 404（已实测确认）。TrimRight 后
// 不以 /v1 结尾则自动补全；已含 /v1（含自定义镜像路径 /api/v1）则原样返回。
func ensureV1Suffix(base string) string {
	b := strings.TrimRight(strings.TrimSpace(base), "/")
	if b == "" {
		return b
	}
	if !strings.HasSuffix(b, "/v1") {
		b += "/v1"
	}
	return b
}

// --- 硅基流动在线模型库 ---

func (d *Daemon) handleProviderFetchSiliconFlowModels(_ context.Context, params json.RawMessage) (any, error) {
	var p rpc.FetchSiliconFlowModelsParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}

	provider, err := resolveProviderConfig(d, p.Provider)
	if err != nil {
		return nil, err
	}

	apiKey := resolveProviderAPIKey(d, provider)
	if apiKey == "" {
		return nil, fmt.Errorf("供应商 %q 尚未配置 API Key，请先在供应商设置中填写", p.Provider)
	}

	// 硅基流动 /v1/models 需要 Bearer Key；只取对话类模型（type=text & sub_type=chat）。
	// 注意：该接口只返回 id/object/created/owned_by，没有价格与免费标识。
	// 用户配置的 base_url 常省略 /v1（如 https://api.siliconflow.cn），需规范化补全。
	base := ensureV1Suffix(provider.BaseURL)
	if base == "" {
		base = "https://api.siliconflow.cn/v1"
	}

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest(http.MethodGet, base+"/models?type=text&sub_type=chat", nil)
	if err != nil {
		return nil, fmt.Errorf("构造硅基流动请求失败: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("无法连接硅基流动服务: %w", err)
	}
	defer resp.Body.Close()

	// base_url 为用户可配置项，LimitReader 防止故障/恶意端点返回超大响应耗尽内存（上限 10MB）
	body, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return nil, fmt.Errorf("读取硅基流动响应失败: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("硅基流动返回错误 (HTTP %d): %s", resp.StatusCode, string(body))
	}

	var sfResp struct {
		Data []struct {
			ID      string `json:"id"`
			OwnedBy string `json:"owned_by"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &sfResp); err != nil {
		return nil, fmt.Errorf("解析硅基流动响应失败: %w", err)
	}

	// 价格补充：/v1/models 只返回 id 等基础信息（官方文档确认），
	// 从价格页内嵌数据解析输入/输出单价（人民币计价，无需汇率）。
	// 解析失败静默降级为 nil：价格留空、模型列表照常返回（已确认的降级策略）。
	pricing := siliconFlowPricing()

	models := make([]rpc.OnlineModelInfo, 0, len(sfResp.Data))
	for _, m := range sfResp.Data {
		if m.ID == "" {
			continue
		}
		info := rpc.OnlineModelInfo{
			ID:      m.ID,
			OwnedBy: m.OwnedBy,
		}
		if p, ok := pricing[strings.ToLower(m.ID)]; ok {
			info.CostPer1MIn = p.InPerM
			info.CostPer1MOut = p.OutPerM
			// 免费判定：输入/输出单价均为 0（价格页刊例价为 0 的免费模型）
			info.Free = p.InPerM == 0 && p.OutPerM == 0
		}
		models = append(models, info)
	}

	return map[string]any{
		"models": models,
		"total":  len(models),
	}, nil
}

// --- 硅基流动价格页解析（/v1/models 不含价格，从价格页内嵌数据补充） ---

// siliconFlowPricePageURL 硅基流动模型价格页（免登录，无需 API Key）。
// 正式控制台域名 cloud.siliconflow.cn/pricing 会 307 跳登录，仅该域名公开可访问。
const siliconFlowPricePageURL = "https://cloud-rd.siliconflow.cn/pricing"

// siliconFlowPrice 单个模型的补充价格（元/M tokens）。
// 页面原始数据即人民币计价，无需汇率换算。
type siliconFlowPrice struct {
	InPerM  float64
	OutPerM float64
}

var (
	sfPricingTable     map[string]siliconFlowPrice // key: strings.ToLower(模型全名)
	sfPricingMu        sync.Mutex
	sfPricingFetchedAt time.Time
	sfPricingFailedAt  time.Time // 失败负缓存：页面改版/网络故障时避免每次打开对话框都重拉约 0.8MB 页面
)

// siliconFlowPricing 拉取并解析硅基流动价格页，返回模型价格表。
// 与汇率缓存同构：成功缓存 1 小时、失败负缓存 5 分钟；网络请求在锁外执行。
// 硅基流动无公开价格 API，价格数据内嵌于价格页 SSR 数据的 pricingApiItems；
// 页面改版导致解析失败时静默返回 nil，调用方按「无价格」降级，不阻断模型列表。
func siliconFlowPricing() map[string]siliconFlowPrice {
	sfPricingMu.Lock()
	if sfPricingTable != nil && time.Since(sfPricingFetchedAt) < time.Hour {
		t := sfPricingTable
		sfPricingMu.Unlock()
		return t
	}
	if !sfPricingFailedAt.IsZero() && time.Since(sfPricingFailedAt) < 5*time.Minute {
		sfPricingMu.Unlock()
		return nil
	}
	sfPricingMu.Unlock()

	table := fetchSiliconFlowPricing()

	sfPricingMu.Lock()
	if table != nil {
		sfPricingTable = table
		sfPricingFetchedAt = time.Now()
	} else {
		sfPricingFailedAt = time.Now()
	}
	sfPricingMu.Unlock()
	return table
}

// fetchSiliconFlowPricing 拉取价格页并聚合为价格表，任一环节失败返回 nil（静默降级）。
func fetchSiliconFlowPricing() map[string]siliconFlowPrice {
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(siliconFlowPricePageURL)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil
	}
	// 页面实测约 0.8MB，LimitReader 兜底防异常膨胀
	body, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return nil
	}
	items, ok := parseSiliconFlowPricingItems(body)
	if !ok {
		return nil
	}
	return aggregateSiliconFlowPricing(items)
}

// siliconFlowPricingItem 是价格页内嵌 pricingApiItems 的单条计费记录。
type siliconFlowPricingItem struct {
	PlaygroundName       string `json:"playgroundName"`       // 模型全名（与 /v1/models 的 id 一致）
	ComponentCode        string `json:"componentCode"`        // 计费项：input-tokens / output-tokens / cached-input-tokens 等
	RealTimePriceCnyUnit string `json:"realTimePriceCnyUnit"` // 单价数值（元/K tokens）
	UnitZhCnName         string `json:"unitZhCnName"`         // 计价单位（如 "K tokens"）
}

// parseSiliconFlowPricingItems 从价格页 HTML 中提取 pricingApiItems 数组。
// 页面为 Next.js SSR，数据内嵌在 flight payload 中（JSON 引号呈 \" 转义形式）：
// 先统一还原引号，再定位数组起点做括号平衡扫描（跳过字符串字面量及其转义）。
func parseSiliconFlowPricingItems(page []byte) ([]siliconFlowPricingItem, bool) {
	normalized := bytes.ReplaceAll(page, []byte(`\"`), []byte(`"`))
	const marker = `"pricingApiItems":[`
	idx := bytes.Index(normalized, []byte(marker))
	if idx < 0 {
		return nil, false
	}
	start := idx + len(marker) - 1 // 指向 '['

	depth, inStr, esc := 0, false, false
	end := -1
	for i := start; i < len(normalized) && end < 0; i++ {
		c := normalized[i]
		if inStr {
			switch {
			case esc:
				esc = false
			case c == '\\':
				esc = true
			case c == '"':
				inStr = false
			}
			continue
		}
		switch c {
		case '"':
			inStr = true
		case '[':
			depth++
		case ']':
			depth--
			if depth == 0 {
				end = i + 1
			}
		}
	}
	if end < 0 {
		return nil, false
	}

	var items []siliconFlowPricingItem
	if json.Unmarshal(normalized[start:end], &items) != nil || len(items) == 0 {
		return nil, false
	}
	return items, true
}

// aggregateSiliconFlowPricing 聚合计费记录为价格表：
// 同一（模型, 计费项）的多条记录（阶梯/峰谷计费）取页面展示顺序的第一条（起步价）；
// 元/K tokens × 1000 换算为元/M tokens。缓存价/音频/图像等计费项不参与。
func aggregateSiliconFlowPricing(items []siliconFlowPricingItem) map[string]siliconFlowPrice {
	type entry struct{ in, out *siliconFlowPricingItem }
	entries := make(map[string]*entry)
	for i := range items {
		it := &items[i]
		if it.ComponentCode != "input-tokens" && it.ComponentCode != "output-tokens" {
			continue
		}
		// 防御：计价单位非 tokens 的记录不参与换算（如音频按秒计费）
		if it.UnitZhCnName != "" && !strings.Contains(it.UnitZhCnName, "tokens") {
			continue
		}
		e := entries[it.PlaygroundName]
		if e == nil {
			e = &entry{}
			entries[it.PlaygroundName] = e
		}
		if it.ComponentCode == "input-tokens" && e.in == nil {
			e.in = it
		} else if it.ComponentCode == "output-tokens" && e.out == nil {
			e.out = it
		}
	}

	table := make(map[string]siliconFlowPrice, len(entries))
	for name, e := range entries {
		var p siliconFlowPrice
		if e.in != nil {
			p.InPerM = parsePrice(e.in.RealTimePriceCnyUnit) * 1000
		}
		if e.out != nil {
			p.OutPerM = parsePrice(e.out.RealTimePriceCnyUnit) * 1000
		}
		table[strings.ToLower(name)] = p
	}
	return table
}

// --- OpenRouter 在线模型库 ---

func (d *Daemon) handleProviderFetchOpenRouterModels(_ context.Context, params json.RawMessage) (any, error) {
	var p rpc.FetchOpenRouterModelsParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}

	// OpenRouter /models 公开接口无需鉴权；base_url 优先取供应商配置（常省略 /v1，需规范化），
	// 为空时兜底官方地址。sort=most-popular 按近一周用量排序，
	// 前端默认视图取前 10 个免费模型。
	// 供应商校验与其它 handler 对齐：不存在时直接报错，避免静默回退官方地址造成数据来源与用户配置不符。
	provider, err := resolveProviderConfig(d, p.Provider)
	if err != nil {
		return nil, err
	}
	base := ensureV1Suffix(provider.BaseURL)
	if base == "" {
		base = "https://openrouter.ai/api/v1"
	}

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(base + "/models?sort=most-popular")
	if err != nil {
		return nil, fmt.Errorf("无法连接 OpenRouter 服务: %w", err)
	}
	defer resp.Body.Close()

	// base_url 为用户可配置项，LimitReader 防止故障/恶意端点返回超大响应耗尽内存（上限 10MB）
	body, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return nil, fmt.Errorf("读取 OpenRouter 响应失败: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OpenRouter 返回错误 (HTTP %d): %s", resp.StatusCode, string(body))
	}

	var orResp struct {
		Data []struct {
			ID            string `json:"id"`
			Name          string `json:"name"`
			Description   string `json:"description"`
			ContextLength int64  `json:"context_length"`
			Pricing       struct {
				Prompt     string `json:"prompt"`
				Completion string `json:"completion"`
			} `json:"pricing"`
			SupportedParameters []string `json:"supported_parameters"`
			Architecture        struct {
				InputModalities []string `json:"input_modalities"`
			} `json:"architecture"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &orResp); err != nil {
		return nil, fmt.Errorf("解析 OpenRouter 响应失败: %w", err)
	}

	// 付费模型成本换算需要 CNY 汇率；获取失败时降级为 0（成本留空，不阻断浏览与添加）
	var cnyRate float64
	hasPaid := false
	for _, m := range orResp.Data {
		if parsePrice(m.Pricing.Prompt) > 0 || parsePrice(m.Pricing.Completion) > 0 {
			hasPaid = true
			break
		}
	}
	if hasPaid {
		cnyRate = usdToCNYRate()
	}

	models := make([]rpc.OnlineModelInfo, 0, len(orResp.Data))
	for _, m := range orResp.Data {
		if m.ID == "" {
			continue
		}
		promptPrice := parsePrice(m.Pricing.Prompt)
		completionPrice := parsePrice(m.Pricing.Completion)
		// 免费判定：输入/输出单价均为 0（OpenRouter 免费模型价格为 "0"）
		free := promptPrice == 0 && completionPrice == 0

		info := rpc.OnlineModelInfo{
			ID:            m.ID,
			Title:         m.Name,
			Description:   m.Description,
			ContextLength: m.ContextLength,
			Free:          free,
			FuncCalling:   slices.Contains(m.SupportedParameters, "tools"),
			Visioning:     slices.Contains(m.Architecture.InputModalities, "image"),
		}
		if !free {
			// OpenRouter 价格单位为 USD/token，×1e6 换算为 USD/M，再乘汇率得 ¥/M
			info.CostPer1MIn = promptPrice * 1e6 * cnyRate
			info.CostPer1MOut = completionPrice * 1e6 * cnyRate
		}
		models = append(models, info)
	}

	return map[string]any{
		"models": models,
		"total":  len(models),
	}, nil
}

// parsePrice 解析 OpenRouter 的字符串价格（"0"、"0.0000015"），
// 非法或负值返回 0。
func parsePrice(s string) float64 {
	v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil || v < 0 {
		return 0
	}
	return v
}

// --- 阿里云百炼在线模型库 ---

func (d *Daemon) handleProviderFetchDashScopeModels(_ context.Context, params json.RawMessage) (any, error) {
	var p rpc.FetchDashScopeModelsParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}

	provider, err := resolveProviderConfig(d, p.Provider)
	if err != nil {
		return nil, err
	}

	apiKey := resolveProviderAPIKey(d, provider)
	if apiKey == "" {
		return nil, fmt.Errorf("供应商 %q 尚未配置 API Key，请先在供应商设置中填写", p.Provider)
	}

	// 百炼原生模型列表 API（/api/v1/models，已实测无需 WorkspaceId 前缀即可用默认业务空间调用），
	// 返回名称/描述/上下文/能力/人民币定价等完整元数据，远比 OpenAI 兼容端点（仅返回 id）丰富。
	// capabilities=TG 只取文本生成模型（MindX 场景）；page_size=100 一次拉全量供前端过滤。
	// 模型列表端点与兼容模式 base_url（.../compatible-mode）路径完全不同，仅复用其 host。
	endpoint := "https://dashscope.aliyuncs.com/api/v1/models"
	if u, err := url.Parse(strings.TrimRight(provider.BaseURL, "/")); err == nil && u.Host != "" {
		endpoint = u.Scheme + "://" + u.Host + "/api/v1/models"
	}

	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest(http.MethodGet, endpoint+"?capabilities=TG&page_no=1&page_size=100", nil)
	if err != nil {
		return nil, fmt.Errorf("构造百炼请求失败: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("无法连接阿里云百炼服务: %w", err)
	}
	defer resp.Body.Close()

	// base_url 为用户可配置项，LimitReader 防止故障/恶意端点返回超大响应耗尽内存（上限 10MB）
	body, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return nil, fmt.Errorf("读取百炼响应失败: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("阿里云百炼返回错误 (HTTP %d): %s", resp.StatusCode, string(body))
	}

	var dsResp struct {
		Success bool `json:"success"`
		Output  struct {
			Models []dashScopeModel `json:"models"`
		} `json:"output"`
	}
	if err := json.Unmarshal(body, &dsResp); err != nil {
		return nil, fmt.Errorf("解析百炼响应失败: %w", err)
	}
	if !dsResp.Success {
		return nil, fmt.Errorf("阿里云百炼返回失败: %s", string(body))
	}

	models := make([]rpc.OnlineModelInfo, 0, len(dsResp.Output.Models))
	for _, m := range dsResp.Output.Models {
		if m.Model == "" {
			continue
		}
		inputPrice := dashScopeTokenPrice(m.Prices, "input_token")
		outputPrice := dashScopeTokenPrice(m.Prices, "output_token")
		// 免费判定：输入/输出单价均为 0（如新用户免费额度的限时模型）
		free := inputPrice == 0 && outputPrice == 0

		models = append(models, rpc.OnlineModelInfo{
			ID:            m.Model,
			Title:         m.Name,
			Description:   m.Description,
			ContextLength: m.ModelInfo.ContextWindow,
			Free:          free,
			// 百炼 token 计价单位即「每百万 tokens」（官方文档示例确认），无需换算
			CostPer1MIn:  inputPrice,
			CostPer1MOut: outputPrice,
			FuncCalling:  slices.Contains(m.Features, "function-calling"),
			Visioning:    slices.Contains(m.InferenceMetadata.RequestModality, "Image"),
		})
	}

	return map[string]any{
		"models": models,
		"total":  len(models),
	}, nil
}

// dashScopeModel 是百炼 /api/v1/models 返回的单个模型条目。
type dashScopeModel struct {
	Model       string   `json:"model"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Features    []string `json:"features"`
	Prices      []struct {
		RangeName string `json:"range_name"`
		Prices    []struct {
			Type  string `json:"type"`
			Price string `json:"price"`
		} `json:"prices"`
	} `json:"prices"`
	InferenceMetadata struct {
		RequestModality []string `json:"request_modality"`
	} `json:"inference_metadata"`
	ModelInfo struct {
		ContextWindow int64 `json:"context_window"`
	} `json:"model_info"`
}

// dashScopeTokenPrice 从百炼分段定价中提取指定计费项（input_token/output_token）
// 的每百万 tokens 单价。优先取 Default 分段，无 Default 时取第一个分段；
// 图像/视频等按张计费的模型没有对应计费项，返回 0。
func dashScopeTokenPrice(prices []struct {
	RangeName string `json:"range_name"`
	Prices    []struct {
		Type  string `json:"type"`
		Price string `json:"price"`
	} `json:"prices"`
}, tokenType string) float64 {
	if len(prices) == 0 {
		return 0
	}
	group := prices[0]
	for _, g := range prices {
		if g.RangeName == "Default" {
			group = g
			break
		}
	}
	for _, item := range group.Prices {
		if item.Type == tokenType {
			return parsePrice(item.Price)
		}
	}
	return 0
}

// --- 智谱在线模型库 ---

func (d *Daemon) handleProviderFetchBigModelModels(_ context.Context, params json.RawMessage) (any, error) {
	var p rpc.FetchBigModelModelsParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}

	provider, err := resolveProviderConfig(d, p.Provider)
	if err != nil {
		return nil, err
	}

	apiKey := resolveProviderAPIKey(d, provider)
	if apiKey == "" {
		return nil, fmt.Errorf("供应商 %q 尚未配置 API Key，请先在供应商设置中填写", p.Provider)
	}

	// 智谱 OpenAI 兼容模型列表端点（{base}/models，需 Bearer Key，已实测）。
	// 与硅基流动一样只返回 id 等基础信息，无价格与免费标识。
	base := strings.TrimRight(provider.BaseURL, "/")
	// 官方域名但路径不完整时规范化为完整 API 根路径；自定义镜像 host 按用户配置原样拼接
	if base == "" || (strings.Contains(base, "open.bigmodel.cn") && !strings.HasSuffix(base, "/api/paas/v4")) {
		base = "https://open.bigmodel.cn/api/paas/v4"
	}

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest(http.MethodGet, base+"/models", nil)
	if err != nil {
		return nil, fmt.Errorf("构造智谱请求失败: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("无法连接智谱服务: %w", err)
	}
	defer resp.Body.Close()

	// base_url 为用户可配置项，LimitReader 防止故障/恶意端点返回超大响应耗尽内存（上限 10MB）
	body, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return nil, fmt.Errorf("读取智谱响应失败: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("智谱返回错误 (HTTP %d): %s", resp.StatusCode, string(body))
	}

	var bmResp struct {
		Data []struct {
			ID      string `json:"id"`
			OwnedBy string `json:"owned_by"`
		} `json:"data"`
		Success *bool `json:"success"`
	}
	if err := json.Unmarshal(body, &bmResp); err != nil {
		return nil, fmt.Errorf("解析智谱响应失败: %w", err)
	}
	// 智谱在 OpenAI 兼容结构外包裹 success 字段，false 时为业务失败
	if bmResp.Success != nil && !*bmResp.Success {
		return nil, fmt.Errorf("智谱返回失败: %s", string(body))
	}

	models := make([]rpc.OnlineModelInfo, 0, len(bmResp.Data))
	for _, m := range bmResp.Data {
		if m.ID == "" {
			continue
		}
		// 接口无价格/免费标识，Free 恒为 false；前端默认展示前 10 个模型
		models = append(models, rpc.OnlineModelInfo{
			ID:      m.ID,
			OwnedBy: m.OwnedBy,
		})
	}

	return map[string]any{
		"models": models,
		"total":  len(models),
	}, nil
}

// --- 腾讯云 TokenHub 在线模型库 ---

func (d *Daemon) handleProviderFetchTencentModels(_ context.Context, params json.RawMessage) (any, error) {
	var p rpc.FetchTencentModelsParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}

	provider, err := resolveProviderConfig(d, p.Provider)
	if err != nil {
		return nil, err
	}

	apiKey := resolveProviderAPIKey(d, provider)
	if apiKey == "" {
		return nil, fmt.Errorf("供应商 %q 尚未配置 API Key，请先在供应商设置中填写", p.Provider)
	}

	// 腾讯云 TokenHub（大模型服务平台）OpenAI 兼容模型列表端点：
	// GET /v1/models（需 Bearer Key，已实测），返回 id/name/status 等基础信息，
	// 无价格与免费标识。status=online 为当前可用模型，pre-offline（即将下线）过滤掉，
	// 避免用户添加后很快失效。base_url 常省略 /v1，需规范化补全。
	base := ensureV1Suffix(provider.BaseURL)
	if base == "" {
		base = "https://tokenhub.tencentmaas.com/v1"
	}

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest(http.MethodGet, base+"/models", nil)
	if err != nil {
		return nil, fmt.Errorf("构造腾讯云请求失败: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("无法连接腾讯云 TokenHub 服务: %w", err)
	}
	defer resp.Body.Close()

	// base_url 为用户可配置项，LimitReader 防止故障/恶意端点返回超大响应耗尽内存（上限 10MB）
	body, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return nil, fmt.Errorf("读取腾讯云响应失败: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("腾讯云 TokenHub 返回错误 (HTTP %d): %s", resp.StatusCode, string(body))
	}

	var thResp struct {
		Data []struct {
			ID     string `json:"id"`
			Name   string `json:"name"`
			Status string `json:"status"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &thResp); err != nil {
		return nil, fmt.Errorf("解析腾讯云响应失败: %w", err)
	}

	models := make([]rpc.OnlineModelInfo, 0, len(thResp.Data))
	for _, m := range thResp.Data {
		if m.ID == "" || (m.Status != "" && m.Status != "online") {
			continue
		}
		// 接口无价格/免费标识，Free 恒为 false；name 为展示名
		models = append(models, rpc.OnlineModelInfo{
			ID:    m.ID,
			Title: m.Name,
		})
	}

	return map[string]any{
		"models": models,
		"total":  len(models),
	}, nil
}

// --- USD→CNY 汇率（免费免 Key 源，带进程级缓存） ---

var (
	fxRateMu        sync.Mutex
	fxRateUSDCNY    float64
	fxRateFetchedAt time.Time
	fxRateFailedAt  time.Time // 最近一次获取失败时间，用于负缓存（避免汇率源不可达时每次请求都阻塞）
)

// usdToCNYRate 获取 USD→CNY 汇率，成功结果 1 小时缓存、失败结果 5 分钟负缓存。
// 网络请求在锁外执行，锁内只做缓存读写，避免并发调用被 5 秒级超时串行阻塞。
// 主源 open.er-api.com（免 Key，每日更新），备源 api.frankfurter.dev（ECB 官方数据）。
// 两个源都失败时返回 0，由调用方降级处理（成本留 0，不阻断添加流程）。
func usdToCNYRate() float64 {
	// 锁内检查缓存命中（成功缓存 1 小时 / 失败缓存 5 分钟）
	fxRateMu.Lock()
	if fxRateUSDCNY > 0 && time.Since(fxRateFetchedAt) < time.Hour {
		r := fxRateUSDCNY
		fxRateMu.Unlock()
		return r
	}
	if !fxRateFailedAt.IsZero() && time.Since(fxRateFailedAt) < 5*time.Minute {
		fxRateMu.Unlock()
		return 0
	}
	fxRateMu.Unlock()

	// 锁外请求：并发时可能重复拉取，桌面低频场景可接受
	client := &http.Client{Timeout: 5 * time.Second}
	var rate float64
	if r := fetchRateERAPI(client); r > 0 {
		rate = r
	} else if r := fetchRateFrankfurter(client); r > 0 {
		rate = r
	}

	fxRateMu.Lock()
	if rate > 0 {
		fxRateUSDCNY = rate
		fxRateFetchedAt = time.Now()
	} else {
		fxRateFailedAt = time.Now()
	}
	fxRateMu.Unlock()
	return rate
}

// fetchRateERAPI 主源：https://open.er-api.com/v6/latest/USD
// 响应形如 {"result":"success","rates":{"CNY":7.09,...}}
func fetchRateERAPI(client *http.Client) float64 {
	resp, err := client.Get("https://open.er-api.com/v6/latest/USD")
	if err != nil {
		return 0
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return 0
	}
	var parsed struct {
		Result string `json:"result"`
		Rates  struct {
			CNY float64 `json:"CNY"`
		} `json:"rates"`
	}
	if json.Unmarshal(body, &parsed) != nil || parsed.Result != "success" {
		return 0
	}
	return parsed.Rates.CNY
}

// fetchRateFrankfurter 备源：https://api.frankfurter.dev/v1/latest?base=USD&symbols=CNY
// 响应形如 {"amount":1.0,"base":"USD","rates":{"CNY":6.7191}}
func fetchRateFrankfurter(client *http.Client) float64 {
	resp, err := client.Get("https://api.frankfurter.dev/v1/latest?base=USD&symbols=CNY")
	if err != nil {
		return 0
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if err != nil {
		return 0
	}
	var parsed struct {
		Rates struct {
			CNY float64 `json:"CNY"`
		} `json:"rates"`
	}
	if json.Unmarshal(body, &parsed) != nil {
		return 0
	}
	return parsed.Rates.CNY
}
