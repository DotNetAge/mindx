package session

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"

	goharnesssession "github.com/DotNetAge/goharness/session"
)

// RoutedSessionStore 是按工作目录分片的会话存储路由器。
//
// 存储布局（TODO 定案：工作目录与会话生命周期绑定，工作目录消失会话即消失）：
//
//	<project_dir>/.sessions/<agent>/<sessionID>/{session.yml, meta.json, tmp/...}
//
// 工作目录清单持久化在独立清单文件（<DataDir>/session_dirs.json，与迁移源目录
// 解耦，避免搬迁完成后旧目录更名 .bak 时把清单一起带走）。路由器实现
// goharness session.SessionStore 全部接口与 mindx 扩展方法，签名与
// FileSessionStore 对齐——上层调用方（RPC handler / client / App）零改动。
//
// 路由规则：
//   - Create：从 SessionOption 重放提取 project_dir 决定落哪个分片（必传，空则报错）；
//   - 按 sessionID 的方法：先查已装载分片，未命中按清单懒装载后重查，仍未命中
//     返回 ErrSessionNotFound（与旧版全树扫描不中语义等价，主链路 Append 前必有
//     成功 Load，此路径不可达）；
//   - ListSessions：各分片并集按 LastActivityAt 排序；工作目录已消失的分片不可见
//     （目录 stat 失败即跳过，不创建）。
type RoutedSessionStore struct {
	mu             sync.Mutex
	registryFile   string                        // 工作目录清单文件（JSON 字符串数组，绝对路径）
	stores         map[string]*FileSessionStore  // 绝对工作目录 → 分片 store（懒装载）
	dirs           []string                      // 登记的工作目录（与 stores 键一致，持久化用）
	slideHandler   goharnesssession.SlideHandler // 新建分片时下发
	tokenEstimator TokenEstimator                // 新建分片时下发
}

// NewRoutedSessionStore 创建路由器。registryFile 为工作目录清单文件路径，
// 所在目录不存在时自动创建；清单文件缺失（首次启动）视为空清单。
func NewRoutedSessionStore(registryFile string) (*RoutedSessionStore, error) {
	if strings.TrimSpace(registryFile) == "" {
		return nil, fmt.Errorf("会话清单文件路径不能为空")
	}
	absFile, err := filepath.Abs(registryFile)
	if err != nil {
		return nil, fmt.Errorf("解析清单文件路径: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(absFile), 0755); err != nil {
		return nil, fmt.Errorf("创建清单目录 %s: %w", filepath.Dir(absFile), err)
	}
	r := &RoutedSessionStore{
		registryFile:   absFile,
		stores:         make(map[string]*FileSessionStore),
		slideHandler:   goharnesssession.NoopSlideHandler,
		tokenEstimator: NewTokenEstimator(),
	}
	r.loadDirs()
	return r, nil
}

// loadDirs 从清单文件装载工作目录列表（不创建分片，分片在按需定位时懒装载）。
// 文件损坏时告警并按空清单处理（不阻塞启动，下次 Create 会重写清单）。
func (r *RoutedSessionStore) loadDirs() {
	data, err := os.ReadFile(r.registryFile)
	if err != nil {
		return // 首次启动无清单属正常
	}
	var dirs []string
	if err := json.Unmarshal(data, &dirs); err != nil {
		log.Printf("[WARN] session: 清单文件解析失败，按空清单处理 %s: %v", r.registryFile, err)
		return
	}
	for _, d := range dirs {
		if d = strings.TrimSpace(d); d != "" {
			r.dirs = append(r.dirs, filepath.Clean(d))
		}
	}
}

// persistDirs 将工作目录清单原子写盘（调用方须持有 r.mu）。
func (r *RoutedSessionStore) persistDirsLocked() error {
	sorted := append([]string(nil), r.dirs...)
	sort.Strings(sorted)
	data, err := json.MarshalIndent(sorted, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal session dirs: %w", err)
	}
	tmp := r.registryFile + ".tmp"
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return fmt.Errorf("write session dirs: %w", err)
	}
	return os.Rename(tmp, r.registryFile)
}

// storeForLocked 返回指定工作目录的分片 store（调用方须持有 r.mu）。
// create=false 时工作目录必须已存在（清单懒装载语义：目录消失则分片不可见）；
// create=true 时允许创建（Create 链路：project_dir 由调用方保证有效）。
func (r *RoutedSessionStore) storeForLocked(absDir string, create bool) (*FileSessionStore, error) {
	if st, ok := r.stores[absDir]; ok {
		return st, nil
	}
	if !create {
		if _, err := os.Stat(absDir); err != nil {
			return nil, nil // 工作目录已消失：静默跳过（调用方按未命中处理）
		}
	}
	st, err := NewFileSessionStore(filepath.Join(absDir, ".sessions"))
	if err != nil {
		return nil, err
	}
	st.SetSlideHandler(r.slideHandler)
	st.SetTokenEstimator(r.tokenEstimator)
	r.stores[absDir] = st
	// 登记进清单（去重）
	if !slices.Contains(r.dirs, absDir) {
		r.dirs = append(r.dirs, absDir)
		if err := r.persistDirsLocked(); err != nil {
			delete(r.stores, absDir)
			return nil, fmt.Errorf("登记工作目录清单失败: %w", err)
		}
	}
	return st, nil
}

// loadAllLocked 返回全部可用分片（已装载 + 清单内目录仍存在者，调用方须持有 r.mu）。
func (r *RoutedSessionStore) loadAllLocked() []*FileSessionStore {
	out := make([]*FileSessionStore, 0, len(r.stores)+len(r.dirs))
	seen := make(map[*FileSessionStore]bool)
	for _, st := range r.stores {
		out = append(out, st)
		seen[st] = true
	}
	for _, d := range r.dirs {
		st, err := r.storeForLocked(d, false)
		if err != nil {
			log.Printf("[WARN] session: 装载工作目录分片失败 %s: %v", d, err)
			continue
		}
		if st != nil && !seen[st] {
			out = append(out, st)
			seen[st] = true
		}
	}
	return out
}

// locate 按 sessionID 定位所在分片：先查已装载分片，未命中按清单懒装载重查。
func (r *RoutedSessionStore) locate(sessionID string) *FileSessionStore {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, st := range r.stores {
		if st.findSessionDir(sessionID) != "" {
			return st
		}
	}
	for _, d := range r.dirs {
		if _, ok := r.stores[d]; ok {
			continue
		}
		st, err := r.storeForLocked(d, false)
		if err != nil {
			log.Printf("[WARN] session: 装载工作目录分片失败 %s: %v", d, err)
			continue
		}
		if st != nil && st.findSessionDir(sessionID) != "" {
			return st
		}
	}
	return nil
}

// RegisterDir 登记一个工作目录（迁移链路回调：搬迁完成后立即写清单，崩溃可重入）。
func (r *RoutedSessionStore) RegisterDir(absProjectDir string) error {
	absDir, err := filepath.Abs(absProjectDir)
	if err != nil {
		return fmt.Errorf("解析工作目录: %w", err)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	_, err = r.storeForLocked(absDir, true)
	return err
}

// Create 新建会话：从 SessionOption 重放提取 project_dir 决定落哪个分片。
func (r *RoutedSessionStore) Create(_ context.Context, agentName string, opts ...goharnesssession.SessionOption) (*goharnesssession.SessionInfo, error) {
	// 重放选项到空 SessionInfo，提取 project_dir（SessionOption 是函数类型，可安全重放）
	probe := &goharnesssession.SessionInfo{}
	for _, opt := range opts {
		opt(probe)
	}
	if strings.TrimSpace(probe.ProjectDir) == "" {
		return nil, fmt.Errorf("创建会话失败: project_dir 不能为空（工作目录分片存储要求会话归属明确）")
	}
	absDir, err := filepath.Abs(probe.ProjectDir)
	if err != nil {
		return nil, fmt.Errorf("解析 project_dir: %w", err)
	}

	r.mu.Lock()
	st, err := r.storeForLocked(absDir, true)
	r.mu.Unlock()
	if err != nil {
		return nil, err
	}

	return st.Create(context.Background(), agentName, opts...)
}

// Append 追加消息（转发到 sessionID 所在分片）。
func (r *RoutedSessionStore) Append(ctx context.Context, sessionID string, agentName string, sponsor string, msg goharnesssession.Message) error {
	st := r.locate(sessionID)
	if st == nil {
		return goharnesssession.ErrSessionNotFound
	}
	return st.Append(ctx, sessionID, agentName, sponsor, msg)
}

// Get 读取全部消息（转发；未命中返回空列表，与旧版全树扫描不中语义一致）。
func (r *RoutedSessionStore) Get(ctx context.Context, sessionID string) ([]goharnesssession.Message, error) {
	st := r.locate(sessionID)
	if st == nil {
		return nil, nil
	}
	return st.Get(ctx, sessionID)
}

// CurrentContext 返回该 agent 全局最近活跃会话的当前上下文窗口
// （跨工作目录取 LastActivityAt 最新者后委托分片实现）。
func (r *RoutedSessionStore) CurrentContext(ctx context.Context, agentName string, maxTokens int64) ([]goharnesssession.Message, error) {
	r.mu.Lock()
	stores := r.loadAllLocked()
	r.mu.Unlock()

	var best *FileSessionStore
	var bestInfo *goharnesssession.SessionInfo
	for _, st := range stores {
		sid, err := st.findSessionByAgent(agentName)
		if err != nil || sid == "" {
			continue
		}
		info, err := st.GetSessionMeta(sid)
		if err != nil {
			continue
		}
		if bestInfo == nil || info.LastActivityAt.After(bestInfo.LastActivityAt) {
			best, bestInfo = st, info
		}
	}
	if best == nil {
		return nil, goharnesssession.ErrSessionNotFound
	}
	return best.CurrentContext(ctx, agentName, maxTokens)
}

// Delete 删除单条消息（转发；未命中返回 nil，与旧版语义一致）。
func (r *RoutedSessionStore) Delete(ctx context.Context, timestamp int64, sessionID string) error {
	st := r.locate(sessionID)
	if st == nil {
		return nil
	}
	return st.Delete(ctx, timestamp, sessionID)
}

// Clear 清空会话消息（转发；未命中返回 nil，与旧版语义一致）。
func (r *RoutedSessionStore) Clear(ctx context.Context, sessionID string) error {
	st := r.locate(sessionID)
	if st == nil {
		return nil
	}
	return st.Clear(ctx, sessionID)
}

// DeleteSession 删除整个会话（转发）。
func (r *RoutedSessionStore) DeleteSession(ctx context.Context, sessionID string) error {
	st := r.locate(sessionID)
	if st == nil {
		return goharnesssession.ErrSessionNotFound
	}
	return st.DeleteSession(ctx, sessionID)
}

// ListSessions 返回全部工作目录分片的会话并集，按 LastActivityAt 降序。
// 工作目录已消失的分片不可见（物理布局直接保证「目录消失会话即消失」）；
// 单个分片列举失败不拖垮全量（容错：任一分片成功即返回部分结果）。
func (r *RoutedSessionStore) ListSessions(ctx context.Context) ([]goharnesssession.SessionInfo, error) {
	r.mu.Lock()
	stores := r.loadAllLocked()
	r.mu.Unlock()

	var all []goharnesssession.SessionInfo
	for _, st := range stores {
		infos, err := st.ListSessions(ctx)
		if err != nil {
			log.Printf("[WARN] session: 列举分片失败，跳过: %v", err)
			continue
		}
		all = append(all, infos...)
	}
	sort.Slice(all, func(i, j int) bool {
		return all[i].LastActivityAt.After(all[j].LastActivityAt)
	})
	return all, nil
}

// GetMeta 读取会话元数据（转发）。
func (r *RoutedSessionStore) GetMeta(_ context.Context, sessionID string) (*goharnesssession.SessionInfo, error) {
	st := r.locate(sessionID)
	if st == nil {
		return nil, goharnesssession.ErrSessionNotFound
	}
	return st.GetMeta(context.Background(), sessionID)
}

// GetSessionMeta 读取会话元数据（mindx 扩展方法，转发）。
func (r *RoutedSessionStore) GetSessionMeta(sessionID string) (*goharnesssession.SessionInfo, error) {
	st := r.locate(sessionID)
	if st == nil {
		return nil, goharnesssession.ErrSessionNotFound
	}
	return st.GetSessionMeta(sessionID)
}

// RenameSession 更新会话标题（mindx 扩展方法，转发）。
func (r *RoutedSessionStore) RenameSession(sessionID string, title string) error {
	st := r.locate(sessionID)
	if st == nil {
		return goharnesssession.ErrSessionNotFound
	}
	return st.RenameSession(sessionID, title)
}

// ResolveSessionDir 返回会话沙箱目录（转发）。
func (r *RoutedSessionStore) ResolveSessionDir(sessionID string) (string, error) {
	st := r.locate(sessionID)
	if st == nil {
		return "", goharnesssession.ErrSessionNotFound
	}
	return st.ResolveSessionDir(sessionID)
}

// GetCursor 读取压缩游标（转发；未命中返回 0，与旧版语义一致）。
func (r *RoutedSessionStore) GetCursor(_ context.Context, sessionID string) (int, error) {
	st := r.locate(sessionID)
	if st == nil {
		return 0, nil
	}
	return st.GetCursor(context.Background(), sessionID)
}

// SetCursor 写入压缩游标（转发）。
func (r *RoutedSessionStore) SetCursor(_ context.Context, sessionID string, cursor int) error {
	st := r.locate(sessionID)
	if st == nil {
		return fmt.Errorf("session %q not found", sessionID)
	}
	return st.SetCursor(context.Background(), sessionID, cursor)
}

// SaveModifyFiles 持久化修改文件追踪（转发）。
func (r *RoutedSessionStore) SaveModifyFiles(sessionID string, files []string) error {
	st := r.locate(sessionID)
	if st == nil {
		return fmt.Errorf("session %q not found", sessionID)
	}
	return st.SaveModifyFiles(sessionID, files)
}

// GetModifyFiles 读取修改文件追踪（转发；未命中返回 nil，与旧版语义一致）。
func (r *RoutedSessionStore) GetModifyFiles(sessionID string) ([]string, error) {
	st := r.locate(sessionID)
	if st == nil {
		return nil, nil
	}
	return st.GetModifyFiles(sessionID)
}

// UpdateMessages 原子替换会话消息（转发）。
func (r *RoutedSessionStore) UpdateMessages(ctx context.Context, sessionID string, cursor int, msgs []goharnesssession.Message) error {
	st := r.locate(sessionID)
	if st == nil {
		return goharnesssession.ErrSessionNotFound
	}
	return st.UpdateMessages(ctx, sessionID, cursor, msgs)
}

// Truncate 截断会话消息（转发；未命中返回 nil，与旧版语义一致）。
func (r *RoutedSessionStore) Truncate(ctx context.Context, sessionID string, keepCount int) error {
	st := r.locate(sessionID)
	if st == nil {
		return nil
	}
	return st.Truncate(ctx, sessionID, keepCount)
}

// SetSlideHandler 设置滑动事件处理器（保存并广播到已装载分片，新建分片时下发）。
func (r *RoutedSessionStore) SetSlideHandler(handler goharnesssession.SlideHandler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.slideHandler = handler
	for _, st := range r.stores {
		st.SetSlideHandler(handler)
	}
}

// SetTokenEstimator 设置 token 估算器（保存并广播到已装载分片，新建分片时下发）。
func (r *RoutedSessionStore) SetTokenEstimator(est TokenEstimator) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if est == nil {
		return
	}
	r.tokenEstimator = est
	for _, st := range r.stores {
		st.SetTokenEstimator(est)
	}
}

// Close 关闭全部分片（FileSessionStore.Close 为空实现，此处保持签名完整）。
func (r *RoutedSessionStore) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, st := range r.stores {
		if err := st.Close(); err != nil {
			return err
		}
	}
	return nil
}
