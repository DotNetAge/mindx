package core

import (
	"bufio"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

// CredentialEntry 是枚举结果里的单条凭据：键名与元信息，不含值。
//
// Meta 由调用方定义语义（如 display、label），机制层只负责随凭据一起存取，
// 不做解释——应用约定与平台实现因此保持分层。
type CredentialEntry struct {
	Key       string            `json:"key"`
	Meta      map[string]string `json:"meta,omitempty"`
	UpdatedAt time.Time         `json:"updated_at"`
}

// CredentialStore 是凭据的机制层：只承诺「按键存取值、删值、带元信息、可枚举」。
// 平台差异（钥匙串 / 加密文件）必须收敛在各实现内部，上层不得依赖任何后端文件
// 形态；需要名字、备注之类的语义，一律通过 Meta 传，不要另建旁挂文件。
type CredentialStore interface {
	Get(key string) (string, error)
	Set(key, value string) error
	Delete(key string) error
	SetMeta(key string, meta map[string]string) error
	List(prefix string) ([]CredentialEntry, error)
}

// serializedStore 给任意后端加一把进程内互斥：凭据索引的读改写与后端自身的
// 全量读写都不是并发安全的（钥匙串 CLI、加密文件共用临时名都会互相踩），
// 而调用方从多个 goroutine 同时读写是常态。后端实现内部只操作原始存储，
// 索引函数拿到的是内层实例，所以不会重复加锁。
type serializedStore struct {
	mu    sync.Mutex
	inner CredentialStore
}

func (s *serializedStore) Get(key string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.inner.Get(key)
}

func (s *serializedStore) Set(key, value string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.inner.Set(key, value)
}

func (s *serializedStore) Delete(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.inner.Delete(key)
}

func (s *serializedStore) SetMeta(key string, meta map[string]string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.inner.SetMeta(key, meta)
}

func (s *serializedStore) List(prefix string) ([]CredentialEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.inner.List(prefix)
}

// credentialIndexKey 是存在同一存储里的索引项：平台后端不一定能枚举已有条目
// （钥匙串就是如此），索引把「有哪些键、各自元信息」记在机制内部，两个后端走
// 同一条代码路径，也就不会为某一平台单独造文件。
const credentialIndexKey = "__mindx_credentials_index_v1"

// warnCorruptIndexOnce 保证索引损坏的提示只出现一次，不让每次读写都刷屏。
var warnCorruptIndexOnce sync.Once

func loadCredentialIndex(store CredentialStore) (map[string]CredentialEntry, error) {
	raw, err := store.Get(credentialIndexKey)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]CredentialEntry{}, nil
		}
		return nil, err
	}
	if raw == "" {
		return map[string]CredentialEntry{}, nil
	}
	idx := map[string]CredentialEntry{}
	if err := json.Unmarshal([]byte(raw), &idx); err == nil {
		return idx, nil
	}
	// 兼容早期写入：macOS 的 security 对含非 ASCII 的值按十六进制回读，
	// 索引里带中文展示名就会变成一长串 hex。能解就解，下面的 Set 会按
	// ASCII 安全形式重写一遍。
	if decoded, err := hex.DecodeString(strings.TrimSpace(raw)); err == nil {
		if err := json.Unmarshal(decoded, &idx); err == nil {
			return idx, nil
		}
	}
	// 解析不出来说明索引已经写坏（并发覆盖、外部手改、磁盘截断都可能）。
	// 索引只承载「有哪些键 + 展示元信息」，凭据值仍在原处，因此不让坏档
	// 永久卡死整个凭据库：退回空索引继续跑，下一次写入会按实际条目重建。
	warnCorruptIndexOnce.Do(func() {
		fmt.Fprintf(os.Stderr, "[credential] 凭据索引损坏，已按空索引继续（条目元信息会逐步重建）\n")
	})
	return map[string]CredentialEntry{}, nil
}

func saveCredentialIndex(store CredentialStore, idx map[string]CredentialEntry) error {
	data, err := json.Marshal(idx)
	if err != nil {
		return err
	}
	// 索引值必须保持纯 ASCII：macOS 的 security 对含非 ASCII 的条目按 hex
	// 回读，写进去的 JSON 下次读回来就不再是 JSON 了。转义后语义不变。
	return store.Set(credentialIndexKey, asciiJSON(string(data)))
}

// asciiJSON 把 JSON 文本里的非 ASCII 字符转成 \uXXXX 转义。
// 非 ASCII 只可能出现在字符串内部，因此逐字符转义是安全的。
func asciiJSON(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r < 0x80 {
			b.WriteRune(r)
			continue
		}
		fmt.Fprintf(&b, "\\u%04x", r)
	}
	return b.String()
}

func touchCredentialIndex(store CredentialStore, key string) error {
	if key == credentialIndexKey {
		return nil
	}
	idx, err := loadCredentialIndex(store)
	if err != nil {
		return err
	}
	e := idx[key]
	e.Key = key
	e.UpdatedAt = time.Now()
	idx[key] = e
	return saveCredentialIndex(store, idx)
}

func setEntryMeta(store CredentialStore, key string, meta map[string]string) error {
	if key == credentialIndexKey {
		return fmt.Errorf("索引键不允许挂元信息")
	}
	idx, err := loadCredentialIndex(store)
	if err != nil {
		return err
	}
	e := idx[key]
	e.Key = key
	if e.Meta == nil {
		e.Meta = map[string]string{}
	}
	for k, v := range meta {
		if v != "" {
			e.Meta[k] = v
		}
	}
	if e.UpdatedAt.IsZero() {
		e.UpdatedAt = time.Now()
	}
	idx[key] = e
	return saveCredentialIndex(store, idx)
}

func listEntries(store CredentialStore, prefix string) ([]CredentialEntry, error) {
	idx, err := loadCredentialIndex(store)
	if err != nil {
		return nil, err
	}
	out := make([]CredentialEntry, 0, len(idx))
	for key, e := range idx {
		if key == credentialIndexKey || !strings.HasPrefix(key, prefix) {
			continue
		}
		e.Key = key
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out, nil
}

func dropEntry(store CredentialStore, key string) error {
	idx, err := loadCredentialIndex(store)
	if err != nil {
		return err
	}
	delete(idx, key)
	return saveCredentialIndex(store, idx)
}

func NewCredentialStore(workspaceDir string) CredentialStore {
	// MINDX_CREDENTIALS_DIR：把凭据文件（密文与本机密钥一起）放到指定目录。
	// 容器里 /home/mindx/.mindx 随镜像重建而清空，挂一个卷到该目录即可让
	// 已保存的密钥跨容器存活；这是显式配置，优先级高于平台默认。
	if dir := strings.TrimSpace(os.Getenv("MINDX_CREDENTIALS_DIR")); dir != "" {
		return &serializedStore{inner: &encryptedFileStore{path: filepath.Join(dir, ".credentials")}}
	}
	switch runtime.GOOS {
	case "darwin":
		return &serializedStore{inner: &macKeychainStore{service: "mindx"}}
	default:
		return &serializedStore{inner: &encryptedFileStore{
			path: filepath.Join(workspaceDir, "settings", ".credentials"),
		}}
	}
}

func ResolveAPIKey(store CredentialStore, ref string) string {
	if store != nil {
		if v, err := store.Get(ref); err == nil && v != "" {
			return v
		}
	}
	return ""
}

// SkillCredentialKey 生成技能凭据存储键 <skill_name>:<env_name>——
// 键带技能名前缀，技能间不共享凭据。MCP 对称复用：<mcp_name>:<env_name>。
func SkillCredentialKey(ownerName, envName string) string {
	return ownerName + ":" + envName
}

// ResolveSkillCredential 技能凭据三层解析链：
// 1. 私有凭据存储（键 <skill_name>:<env_name>，OAuth 令牌为整体 JSON）
// 2. 进程环境变量（envName）
// 3. 均缺失 → 返回 ("", false)，由调用方触发准备流程（OAuth 授权 / 安装确认）
func ResolveSkillCredential(store CredentialStore, skillName, envName string) (string, bool) {
	if store != nil {
		if v, err := store.Get(SkillCredentialKey(skillName, envName)); err == nil && v != "" {
			return v, true
		}
	}
	if v := os.Getenv(envName); v != "" {
		return v, true
	}
	return "", false
}

// memoryCredentialStore 是进程内实现：测试与「只在本次会话可用」的场景使用，
// 与真实后端走同一套索引逻辑，避免测试里另造一套行为。
type memoryCredentialStore struct {
	mu   sync.Mutex
	data map[string]string
}

func (s *memoryCredentialStore) Get(key string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if v, ok := s.data[key]; ok {
		return v, nil
	}
	return "", nil
}

func (s *memoryCredentialStore) Set(key, value string) error {
	s.mu.Lock()
	s.data[key] = value
	s.mu.Unlock()
	return touchCredentialIndex(s, key)
}

func (s *memoryCredentialStore) Delete(key string) error {
	s.mu.Lock()
	delete(s.data, key)
	s.mu.Unlock()
	return dropEntry(s, key)
}

func (s *memoryCredentialStore) SetMeta(key string, meta map[string]string) error {
	return setEntryMeta(s, key, meta)
}

func (s *memoryCredentialStore) List(prefix string) ([]CredentialEntry, error) {
	return listEntries(s, prefix)
}

// NewMemoryCredentialStore 创建进程内凭据库（测试专用，不落盘）。
func NewMemoryCredentialStore() CredentialStore {
	return &serializedStore{inner: &memoryCredentialStore{data: map[string]string{}}}
}

type macKeychainStore struct {
	service string
}

func (s *macKeychainStore) Get(key string) (string, error) {
	out, err := exec.Command("security", "find-generic-password",
		"-s", s.service, "-a", key, "-w").Output()
	if err != nil {
		// 条目不存在时 security 返回 44；把它当「没有这个键」而不是故障，
		// 与加密文件后端保持一致，否则首次写入索引就会失败。
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 44 {
			return "", nil
		}
		return "", fmt.Errorf("keychain get %q: %w", key, err)
	}
	return strings.TrimSpace(string(out)), nil
}

func (s *macKeychainStore) Set(key, value string) error {
	// 密码不进 argv：`-w` 作为最后一个参数时 security 会从 stdin 读两遍
	// （密码 + 确认），而同机其他进程能看到命令行参数。含换行的值无法用
	// 这种方式安全传输，直接拒绝，不让它把两行拼成「确认不一致」。
	if strings.ContainsAny(value, "\r\n") {
		return fmt.Errorf("keychain set %q: 凭据值不能包含换行", key)
	}
	cmd := exec.Command("security", "add-generic-password",
		"-s", s.service, "-a", key, "-U", "-w")
	cmd.Stdin = strings.NewReader(value + "\n" + value + "\n")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("keychain set %q: %w（%s）", key, err, strings.TrimSpace(string(out)))
	}
	// 同上：索引写入失败不让主写入失败。
	_ = touchCredentialIndex(s, key)
	return nil
}

func (s *macKeychainStore) Delete(key string) error {
	// 不存在（exit 44）按成功处理：删除是幂等操作，上层不该因「本来就没有」
	// 而报错；但钥匙串被锁、磁盘/权限故障要如实上报，否则界面会显示「已清除」
	// 而条目仍在。
	err := exec.Command("security", "delete-generic-password",
		"-s", s.service, "-a", key).Run()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 44 {
			_ = dropEntry(s, key)
			return nil
		}
		return fmt.Errorf("keychain delete %q: %w", key, err)
	}
	_ = dropEntry(s, key)
	return nil
}

func (s *macKeychainStore) SetMeta(key string, meta map[string]string) error {
	return setEntryMeta(s, key, meta)
}

func (s *macKeychainStore) List(prefix string) ([]CredentialEntry, error) {
	return listEntries(s, prefix)
}

type encryptedFileStore struct {
	path string
}

func (s *encryptedFileStore) Get(key string) (string, error) {
	data, err := s.readEncrypted()
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}

	scanner := bufio.NewScanner(strings.NewReader(data))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 && parts[0] == key {
			return parts[1], nil
		}
	}
	return "", nil
}

func (s *encryptedFileStore) Set(key, value string) error {
	existing := make(map[string]string)
	data, err := s.readEncrypted()
	if err != nil && !os.IsNotExist(err) {
		// 读失败（损坏/无法解密/权限抖动）时既不能把空表当基线覆盖写，
		// 否则其余全部凭据会被静默清空、且无任何痕迹。宁可让本次写入失败。
		return fmt.Errorf("现有凭据读不出来，为避免把其它凭据一起清空，这次保存已取消。可以先删除凭据文件后重新登录，或从备份恢复（原因：%w）", err)
	}
	if err == nil {
		scanner := bufio.NewScanner(strings.NewReader(data))
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				existing[parts[0]] = parts[1]
			}
		}
	}

	existing[key] = value

	var b strings.Builder
	for k, v := range existing {
		b.WriteString(k)
		b.WriteString("=")
		b.WriteString(v)
		b.WriteString("\n")
	}

	if err := s.writeEncrypted(b.String()); err != nil {
		return err
	}
	// 索引只是为了让上层能枚举，属附加能力：它失败不得连坐主写入，
	// 否则老代码（模型 API Key、市场令牌、技能凭据）会把「已存好」报成失败。
	_ = touchCredentialIndex(s, key)
	return nil
}

func (s *encryptedFileStore) Delete(key string) error {
	data, err := s.readEncrypted()
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var b strings.Builder
	scanner := bufio.NewScanner(strings.NewReader(data))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 && parts[0] == key {
			continue
		}
		b.WriteString(line)
		b.WriteString("\n")
	}
	if err := s.writeEncrypted(b.String()); err != nil {
		return err
	}
	_ = dropEntry(s, key)
	return nil
}

func (s *encryptedFileStore) SetMeta(key string, meta map[string]string) error {
	return setEntryMeta(s, key, meta)
}

func (s *encryptedFileStore) List(prefix string) ([]CredentialEntry, error) {
	return listEntries(s, prefix)
}

// keyFilePath 本机密钥落点：与密文同目录、名字固定后缀 .key。
func (s *encryptedFileStore) keyFilePath() string {
	return s.path + ".key"
}

// localKey 读取或生成本机随机密钥（32 字节，0600）。
//
// 为什么不能继续用「主机名 + 系统 + 架构 + 固定常量」派生：那是可离线复算的
// 公开信息，任何拿到密文文件的进程/备份/同步副本，只要知道主机名就能解出
// API Key、账号口令与市场令牌，所谓「加密存储」等价于简单编码。改成随机密钥
// 后，密文与密钥必须同时到手才可解。
//
// 密钥与密文同目录：能读到密文的本地进程通常也读得到密钥，这挡住的是
// 「只漏了密文」的场景（同步盘、备份、日志附件、误提交），换机也不再解不开。
// 更强的隔离需要系统钥匙串（macOS 走 macKeychainStore，已不需要此文件）。
func (s *encryptedFileStore) localKey() ([]byte, error) {
	path := s.keyFilePath()
	if data, err := os.ReadFile(path); err == nil {
		if len(data) == 32 {
			return data, nil
		}
		return nil, fmt.Errorf("本机密钥文件长度异常（%d 字节，应为 32）: %s", len(data), path)
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("读取本机密钥失败: %w", err)
	}

	key := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, fmt.Errorf("生成本机密钥失败: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, key, 0600); err != nil {
		return nil, fmt.Errorf("保存本机密钥失败: %w", err)
	}
	return key, nil
}

// legacyKey 旧版密钥派生（主机名 + 系统 + 架构 + 固定常量）。
// 仅供解密迁移前的存量密文，新密文一律用 localKey；存量密文在首次成功
// 读取时即被新密钥重写（见 readEncrypted），此后该派生不再参与加解密。
func (s *encryptedFileStore) legacyKey() []byte {
	hostname, _ := os.Hostname()
	machineID := hostname + runtime.GOOS + runtime.GOARCH
	hash := sha256.Sum256([]byte(machineID + "mindx-credential-v1"))
	return hash[:]
}

func (s *encryptedFileStore) readEncrypted() (string, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return "", err
	}

	if plaintext, err := s.decrypt(data); err == nil {
		return string(plaintext), nil
	}

	// 旧密钥密文（升级前落盘）：能解开就用新随机密钥重写，把历史弱点一次
	// 迁走。重写失败不影响本次读取，下次读取仍可走同一分支。
	if plaintext, legacyErr := decryptWith(s.legacyKey(), data); legacyErr == nil {
		_ = s.writeEncrypted(string(plaintext))
		return string(plaintext), nil
	}

	// 两条密钥都解不开：文件损坏或来自其它机器（密钥不同）。绝不把密文
	// 当明文继续解析——那等于凭据静默全丢且无任何痕迹；也不返回空结果，
	// 否则「密钥漂移/文件损坏」会被上层当成「没有凭据」，静默进入重新授权。
	// 备份坏文件（仅首次，保留诊断线索）后返回明确错误。
	backup := s.path + ".corrupt"
	if _, statErr := os.Stat(backup); os.IsNotExist(statErr) {
		_ = os.WriteFile(backup, data, 0600)
	}
	return "", fmt.Errorf("凭据文件无法解密（文件损坏，或来自另一台机器）：%s", s.path)
}

func (s *encryptedFileStore) writeEncrypted(plaintext string) error {
	encrypted, err := s.encrypt([]byte(plaintext))
	if err != nil {
		return err
	}

	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	// 先写临时文件再改名：写到一半被打断（容器被 kill、磁盘满）时，
	// 旧密文仍然完整可用，不会把全部凭据变成解不开的残片。临时名必须唯一，
	// 固定名会让两个写者交叉覆盖同一份密文。
	tmp, err := os.CreateTemp(dir, filepath.Base(s.path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	cleanup := func() {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
	}
	if err := tmp.Chmod(0600); err != nil {
		cleanup()
		return err
	}
	if _, err := tmp.Write(encrypted); err != nil {
		cleanup()
		return err
	}
	// 先落盘再改名：rename 只保证目录项原子，不保证数据已经写到介质。
	if err := tmp.Sync(); err != nil {
		cleanup()
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, s.path); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	return nil
}

func (s *encryptedFileStore) encrypt(plaintext []byte) ([]byte, error) {
	key, err := s.localKey()
	if err != nil {
		return nil, err
	}
	return encryptWith(key, plaintext)
}

func (s *encryptedFileStore) decrypt(ciphertext []byte) ([]byte, error) {
	key, err := s.localKey()
	if err != nil {
		return nil, err
	}
	return decryptWith(key, ciphertext)
}

// encryptWith 用给定密钥做 AES-256-GCM 封装（nonce 前置）。
func encryptWith(key, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	return aead.Seal(nonce, nonce, plaintext, nil), nil
}

// decryptWith 用给定密钥解开 AES-256-GCM 密文。
func decryptWith(key, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	if len(ciphertext) < aead.NonceSize() {
		return nil, fmt.Errorf("data too short for AES-GCM")
	}

	nonce, body := ciphertext[:aead.NonceSize()], ciphertext[aead.NonceSize():]
	plaintext, err := aead.Open(nil, nonce, body, nil)
	if err != nil {
		return nil, err
	}

	return plaintext, nil
}
