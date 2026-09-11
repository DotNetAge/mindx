package mcp

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/DotNetAge/goharness/tools"
	"golang.org/x/sync/errgroup"
)

// DefaultMarketMCPURL 是 COS 上 market 中 mcp.json 的默认地址。
// 与 bundle.DefaultMarketManifestURL 同 bucket（repo-1257961037）。
const DefaultMarketMCPURL = "https://repo-1257961037.cos.ap-guangzhou.myqcloud.com/market/mcp.json"

// syncHTTPClient 同步用的 HTTP client（独立于 bundle.MarketClient，更轻量）。
const syncHTTPClientTimeout = 15 * time.Second

// enabledDiscoverTimeout EnabledTools() 内部 tools/list 的总超时预算。
const enabledDiscoverTimeout = 15 * time.Second

// toolsCacheTTL 工具清单缓存 TTL。
// MCP server 的工具列表通常稳定（除非 server 升级或用户手动变更），
// 5 分钟内重复 createRuntime 直接复用缓存，省掉 N 次串行 tools/list。
const toolsCacheTTL = 5 * time.Minute

// Manager is the top-level orchestrator for MCP integration.
// It owns the connection pool, mcp.json file path, and credential store.
// 所有 server 配置和工具清单都来自 mcp.json + 运行时 MCP tools/list，
// 不再依赖任何外部存储（bbolt / SQLite 等）。
type Manager struct {
	pool       *ConnectionPool
	credStore  CredentialStore
	rpc        *RPCHandler
	log        Logger
	configPath string // mcp.json 绝对路径

	// marketURL 是远程 mcp.json 的下载地址，空字符串时 Sync() 跳过。
	marketURL string
	httpCli   *http.Client // 复用，非 nil

	// ── 热更新回调 ────────────────────────────────────────────────────────
	// 当 server 配置发生变更（SetEnabled / AddServer / RemoveServer / Sync）时触发。
	// 典型用途：App 侧注册 InvalidateRuntimes，使下一轮对话的 Runtime 重新装配工具。
	// nil 时跳过（不回调）。由 daemon 组装时 SetOnConfigChanged() 注入。
	onConfigChangedMu sync.RWMutex
	onConfigChanged   func()

	// ── 工具清单缓存 ──────────────────────────────────────────────────────
	// 工具发现结果（BuildTools 后的 []FuncTool）缓存，避免每次 EnabledTools() 都串行拉。
	// SetEnabled / AddServer / RemoveServer / Sync 成功后必须 InvalidateToolsCache()。
	toolsCacheMu sync.RWMutex
	toolsCache   []tools.FuncTool
	toolsCacheAt time.Time
}

// NewManager 创建 MCP Manager 并加载 mcp.json。
// configPath 是 mcp.json 的绝对路径（通常 ~/.mindx/settings/mcp.json）。
// marketURL 可选：传入 COS 上 mcp.json 的地址，Sync() 才能工作；空串或 "-" 时跳过远程同步。
func NewManager(log Logger, credStore CredentialStore, configPath, marketURL string) *Manager {
	if log == nil {
		log = nopLogger{}
	}
	if marketURL == "-" {
		marketURL = ""
	}
	m := &Manager{
		credStore:  credStore,
		log:        log,
		configPath: configPath,
		marketURL:  marketURL,
		httpCli:    &http.Client{Timeout: syncHTTPClientTimeout},
	}

	m.pool = NewConnectionPool(log, credStore)
	m.rpc = NewRPCHandler(m)

	servers, err := LoadServers(configPath)
	if err != nil {
		m.log.Error("mcp: 读取 mcp.json 失败", err, "path", configPath)
	} else if len(servers) > 0 {
		m.log.Info("mcp: 从 mcp.json 加载 server 配置", "count", len(servers), "path", configPath)
		for _, s := range servers {
			m.log.Debug("mcp: server", "name", s.Name, "type", string(s.Type), "title", s.Title, "enabled", s.Enabled)
		}
	} else {
		m.log.Info("mcp: mcp.json 为空或不存在，等待用户配置", "path", configPath)
	}
	_ = m.pool.LoadConfig(context.Background(), configPath)

	m.pool.StartReapLoop(30 * time.Second)
	m.log.Info("mcp: manager initialized", "reap_interval", "30s", "tools_cache_ttl", toolsCacheTTL)

	return m
}

// RPCHandler returns the JSON-RPC handler for WebUI configuration.
func (m *Manager) RPCHandler() *RPCHandler { return m.rpc }

// ConfigPath 返回 mcp.json 路径，供外部（如 handler）直接读取时使用。
func (m *Manager) ConfigPath() string { return m.configPath }

// MarketURL 返回远程 mcp.json 的地址（空串表示未配置远程源）。
func (m *Manager) MarketURL() string { return m.marketURL }

// ── 缓存失效 ────────────────────────────────────────────────────────────────

// SetOnConfigChanged 注册一个回调，在 server 配置变更后触发。
// daemon.go 组装时注入 app.InvalidateRuntimes，使 Runtime 下一轮对话重新装配工具。
// 传 nil 清除回调。
func (m *Manager) SetOnConfigChanged(fn func()) {
	m.onConfigChangedMu.Lock()
	m.onConfigChanged = fn
	m.onConfigChangedMu.Unlock()
}

// InvalidateToolsCache 强制清空工具清单缓存 + 触发热更新回调。
// 必须在 server 配置变更后调用（SetEnabled / AddServer / RemoveServer / Sync）。
// Shutdown 中不调用（Shutdown 直接操作字段，不走此入口）。
func (m *Manager) InvalidateToolsCache() {
	m.toolsCacheMu.Lock()
	m.toolsCache = nil
	m.toolsCacheAt = time.Time{}
	m.toolsCacheMu.Unlock()
	m.log.Debug("mcp: 工具清单缓存已失效")

	// 触发热更新回调（App 侧注册了 InvalidateRuntimes）
	m.onConfigChangedMu.RLock()
	fn := m.onConfigChanged
	m.onConfigChangedMu.RUnlock()
	if fn != nil {
		m.log.Debug("mcp: 触发 onConfigChanged（Runtime 将在下一轮对话重建）")
		fn()
	}
}

// ── 远程同步 ────────────────────────────────────────────────────────────────

// SyncResult 描述一次 Sync() 的结果，供 RPC 返回给前端展示。
type SyncResult struct {
	RemoteURL  string   `json:"remote_url"`
	Downloaded bool     `json:"downloaded"`
	Added      []string `json:"added,omitempty"`
	Error      string   `json:"error,omitempty"`
	Reloaded   bool     `json:"reloaded"`
}

// Sync 把远程 COS 上的 mcp.json 合并进本地。
// 触发点：WebUI 进入 MCP 浏览器时调 mcp.sync RPC。
func (m *Manager) Sync(ctx context.Context) SyncResult {
	result := SyncResult{RemoteURL: m.marketURL}

	if m.marketURL == "" {
		result.Error = "未配置远程 mcp.json 地址"
		return result
	}

	data, err := m.download(ctx, m.marketURL)
	if err != nil {
		result.Error = fmt.Sprintf("下载远程 mcp.json 失败: %v", err)
		m.log.Warn("mcp: Sync 下载失败", "error", err, "url", m.marketURL)
		return result
	}
	result.Downloaded = true

	added, err := MergeRemote(m.configPath, data)
	if err != nil {
		result.Error = fmt.Sprintf("合并远程条目失败: %v", err)
		m.log.Error("mcp: Sync 合并失败", err)
		return result
	}
	result.Added = added

	if len(added) > 0 {
		servers, _ := LoadServers(m.configPath)
		for _, s := range servers {
			for _, a := range added {
				if s.Name == a && s.Enabled {
					m.pool.AddServer(s)
					break
				}
			}
		}
		result.Reloaded = true
		m.log.Info("mcp: Sync 完成，新增 %d 个 server", "count", len(added), "servers", strings.Join(added, ","))
		m.InvalidateToolsCache()
	} else {
		m.log.Debug("mcp: Sync 完成，无新增条目")
	}

	return result
}

// download HTTP GET 下载远程 mcp.json，带超时和大小上限。
func (m *Manager) download(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := m.httpCli.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	return data, nil
}

// ── 工具发现 & 注册 ────────────────────────────────────────────────────────

// EnabledTools 返回所有 enabled MCP server 的工具，作为 goharness FuncTool 实例。
// Called by Runtime during createRuntime() to register MCP tools.
//
// 缓存策略：toolsCacheTTL 内直接复用上一次 BuildTools 结果；
// 超时才重新 discover。server 配置变更后 InvalidateToolsCache 强制跳过缓存。
//
// 发现阶段：对每个 enabled server 并发调 tools/list（errgroup），失败的 server 跳过，
// 不阻塞其他。总超时 enabledDiscoverTimeout 兜底。
func (m *Manager) EnabledTools() []tools.FuncTool {
	// ── 缓存命中检查 ────────────────────────────────────────────
	m.toolsCacheMu.RLock()
	cached := m.toolsCache
	cachedAt := m.toolsCacheAt
	m.toolsCacheMu.RUnlock()
	if cached != nil && time.Since(cachedAt) < toolsCacheTTL {
		m.log.Debug("mcp: 工具清单缓存命中", "age", time.Since(cachedAt).Round(time.Second), "count", len(cached))
		return cached
	}

	// ── 并发 discover ──────────────────────────────────────────
	ctx, cancel := context.WithTimeout(context.Background(), enabledDiscoverTimeout)
	defer cancel()

	poolServers := m.pool.ActiveServerNames()
	if len(poolServers) == 0 {
		m.log.Debug("mcp: 无 enabled server")
		m.toolsCacheMu.Lock()
		m.toolsCache = nil
		m.toolsCacheAt = time.Now()
		m.toolsCacheMu.Unlock()
		return nil
	}

	type discoverResult struct {
		server string
		defs   []toolDef
		err    error
	}

	resultCh := make(chan discoverResult, len(poolServers))
	g, gctx := errgroup.WithContext(ctx)

	for _, name := range poolServers {
		name := name
		g.Go(func() error {
			defs, err := m.pool.DiscoverTools(gctx, name)
			resultCh <- discoverResult{server: name, defs: defs, err: err}
			return nil // 单个 server discover 失败不阻塞其他
		})
	}
	// 等全部 goroutine 结束（含全部失败）
	_ = g.Wait()
	close(resultCh)

	var all []toolDef
	var failed []string
	for r := range resultCh {
		if r.err != nil {
			m.log.Warn("mcp: discover tools 失败", "server", r.server, "error", r.err)
			failed = append(failed, r.server)
			continue
		}
		for i := range r.defs {
			r.defs[i].server = r.server
		}
		all = append(all, r.defs...)
	}

	toolList := BuildTools(all, m.pool)

	// ── 写缓存 ────────────────────────────────────────────────
	m.toolsCacheMu.Lock()
	m.toolsCache = toolList
	m.toolsCacheAt = time.Now()
	m.toolsCacheMu.Unlock()

	m.log.Info("mcp: 提供可用工具",
		"count", len(toolList),
		"servers", poolServers,
		"failed", failed,
		"cache_ttl", toolsCacheTTL)
	return toolList
}

// SetEnabled 切换 server 的 enabled 属性并写回 mcp.json。
// 热生效：关闭时立即断开并移出连接池；开启时加回连接池（懒连接）。
func (m *Manager) SetEnabled(name string, enabled bool) error {
	servers, err := LoadServers(m.configPath)
	if err != nil {
		return err
	}
	found := false
	for i := range servers {
		if servers[i].Name == name {
			servers[i].Enabled = enabled
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("server %q 不存在", name)
	}
	if err := SaveServers(m.configPath, servers); err != nil {
		m.log.Error("mcp: 写入 mcp.json 失败", err, "server", name)
		return err
	}

	if enabled {
		for _, s := range servers {
			if s.Name == name {
				m.pool.AddServer(s)
			}
		}
	} else {
		m.pool.RemoveServer(name)
	}
	m.InvalidateToolsCache()
	m.log.Info("mcp: server 启用状态已切换", "name", name, "enabled", enabled)
	return nil
}

// AddServer 新增或更新一个 server，立即写回 mcp.json。
func (m *Manager) AddServer(ctx context.Context, cfg ServerConfig) error {
	if err := AddOrUpdateServer(m.configPath, cfg); err != nil {
		m.log.Error("mcp: 写入 mcp.json 失败", err, "server", cfg.Name)
		return err
	}

	m.pool.AddServer(cfg)
	m.InvalidateToolsCache()
	m.log.Info("mcp: server 已更新", "name", cfg.Name, "type", string(cfg.Type))
	return nil
}

// RemoveServer 从 mcp.json 删除 server，同时清理 pool。
func (m *Manager) RemoveServer(name string) error {
	if err := RemoveServerByName(m.configPath, name); err != nil {
		m.log.Error("mcp: 从 mcp.json 删除失败", err, "server", name)
		return err
	}

	m.pool.RemoveServer(name)
	m.InvalidateToolsCache()
	m.log.Info("mcp: server 已删除", "name", name)
	return nil
}

// TestServerConnection 测试某个 server 是否能连通。
// 不管该 server 在 pool 里还是不在（disabled）都能测：
//   - enabled=true → pool 里已有，直接 TestConnection
//   - enabled=false / pool 里没有 → 从 mcp.json 拿 cfg，临时加进 pool 测完再移除
//
// 临时 cfg 不会持久化，pool 里不会残留。
func (m *Manager) TestServerConnection(ctx context.Context, name string) error {
	// 1. 如果 pool 里已有（enabled=true 的正常情况），直接测
	names := m.pool.ActiveServerNames()
	for _, n := range names {
		if n == name {
			return m.pool.TestConnection(ctx, name)
		}
	}

	// 2. pool 里没有（disabled / 从未加载）→ 从 mcp.json 拿 cfg
	cfg, err := m.lookupConfig(name)
	if err != nil {
		return err
	}

	// 3. 临时加进 pool，测完 Disconnect + RemoveServer 清理
	m.pool.AddServer(*cfg)
	defer func() {
		m.pool.Disconnect(name)
		m.pool.RemoveServer(name)
	}()

	return m.pool.TestConnection(ctx, name)
}

// DiscoverServerTools 列出某个 server 的工具清单（支持 disabled server）。
// 原理同 TestServerConnection：不在 pool 里就临时加进去跑完就删。
func (m *Manager) DiscoverServerTools(ctx context.Context, name string) ([]toolDef, error) {
	// 1. 已在 pool 里（enabled），直接 discover
	names := m.pool.ActiveServerNames()
	for _, n := range names {
		if n == name {
			return m.pool.DiscoverTools(ctx, name)
		}
	}

	// 2. 不在 pool → 从 mcp.json 拿 cfg 临时加载
	cfg, err := m.lookupConfig(name)
	if err != nil {
		return nil, err
	}

	m.pool.AddServer(*cfg)
	defer func() {
		m.pool.Disconnect(name)
		m.pool.RemoveServer(name)
	}()

	return m.pool.DiscoverTools(ctx, name)
}

// lookupConfig 从 mcp.json 里按名找 ServerConfig（不管 enabled 值）。
func (m *Manager) lookupConfig(name string) (*ServerConfig, error) {
	servers, err := LoadServers(m.configPath)
	if err != nil {
		return nil, fmt.Errorf("读取 mcp.json 失败: %w", err)
	}
	for i := range servers {
		if servers[i].Name == name {
			return &servers[i], nil
		}
	}
	return nil, fmt.Errorf("server %q 不在 mcp.json 中", name)
}

// Shutdown gracefully closes all connections and stops the reap loop.
// 不触发 onConfigChanged（关闭时 Runtime 不需要重建）。
func (m *Manager) Shutdown() {
	m.log.Info("mcp: 正在关闭")
	m.pool.CloseAll()
	m.toolsCacheMu.Lock()
	m.toolsCache = nil
	m.toolsCacheAt = time.Time{}
	m.toolsCacheMu.Unlock()
	m.log.Info("mcp: 已关闭")
}

// ── noop logger fallback ────────────────────────────────────────────────────

type nopLogger struct{}

func (nopLogger) Info(string, ...any)         {}
func (nopLogger) Error(string, error, ...any) {}
func (nopLogger) Debug(string, ...any)        {}
func (nopLogger) Warn(string, ...any)         {}

var _ Logger = nopLogger{}
