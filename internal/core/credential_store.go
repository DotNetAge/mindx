package core

import (
	"bufio"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

type CredentialStore interface {
	Get(key string) (string, error)
	Set(key, value string) error
}

func NewCredentialStore(workspaceDir string) CredentialStore {
	switch runtime.GOOS {
	case "darwin":
		return &macKeychainStore{service: "mindx"}
	default:
		return &encryptedFileStore{
			path: filepath.Join(workspaceDir, "settings", ".credentials"),
		}
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

type macKeychainStore struct {
	service string
}

func (s *macKeychainStore) Get(key string) (string, error) {
	out, err := exec.Command("security", "find-generic-password",
		"-s", s.service, "-a", key, "-w").Output()
	if err != nil {
		return "", fmt.Errorf("keychain get %q: %w", key, err)
	}
	return strings.TrimSpace(string(out)), nil
}

func (s *macKeychainStore) Set(key, value string) error {
	err := exec.Command("security", "add-generic-password",
		"-s", s.service, "-a", key, "-w", value, "-U").Run()
	if err != nil {
		return fmt.Errorf("keychain set %q: %w", key, err)
	}
	return nil
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

	return s.writeEncrypted(b.String())
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

	return os.WriteFile(s.path, encrypted, 0600)
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
