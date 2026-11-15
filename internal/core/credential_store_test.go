package core

import (
	"os"
	"path/filepath"
	"testing"
)

// TestEncryptedFileStore_Roundtrip 验证加密凭证的写入和读取一致性。
func TestEncryptedFileStore_Roundtrip(t *testing.T) {
	tmpDir := t.TempDir()
	store := &encryptedFileStore{
		path: filepath.Join(tmpDir, "settings", ".credentials"),
	}

	// Set 多个 Key
	if err := store.Set("openai_key", "sk-abc123"); err != nil {
		t.Fatalf("Set failed: %v", err)
	}
	if err := store.Set("anthropic_key", "sk-ant-xyz"); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Get 验证
	val, err := store.Get("openai_key")
	if err != nil {
		t.Fatalf("Get openai_key failed: %v", err)
	}
	if val != "sk-abc123" {
		t.Errorf("openai_key = %q, want %q", val, "sk-abc123")
	}

	val, err = store.Get("anthropic_key")
	if err != nil {
		t.Fatalf("Get anthropic_key failed: %v", err)
	}
	if val != "sk-ant-xyz" {
		t.Errorf("anthropic_key = %q, want %q", val, "sk-ant-xyz")
	}
}

// TestEncryptedFileStore_Update 验证已存在 Key 的更新覆盖。
func TestEncryptedFileStore_Update(t *testing.T) {
	tmpDir := t.TempDir()
	store := &encryptedFileStore{
		path: filepath.Join(tmpDir, "settings", ".credentials"),
	}

	if err := store.Set("mykey", "old_value"); err != nil {
		t.Fatalf("Set initial failed: %v", err)
	}
	if err := store.Set("mykey", "new_value"); err != nil {
		t.Fatalf("Set update failed: %v", err)
	}

	val, err := store.Get("mykey")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if val != "new_value" {
		t.Errorf("after update = %q, want %q", val, "new_value")
	}
}

// TestEncryptedFileStore_Missing 验证读取不存在的 Key 返回空字符串无错误。
func TestEncryptedFileStore_Missing(t *testing.T) {
	tmpDir := t.TempDir()
	store := &encryptedFileStore{
		path: filepath.Join(tmpDir, "settings", ".credentials"),
	}

	val, err := store.Get("nonexistent")
	if err != nil {
		t.Fatalf("Get nonexistent failed: %v", err)
	}
	if val != "" {
		t.Errorf("nonexistent = %q, want empty", val)
	}
}

// TestEncryptedFileStore_EncryptDecrypt 直接测试底层的 encrypt/decrypt 函数。
func TestEncryptedFileStore_EncryptDecrypt(t *testing.T) {
	tmpDir := t.TempDir()
	store := &encryptedFileStore{
		path: filepath.Join(tmpDir, "settings", ".credentials"),
	}

	plaintext := "very-secret-api-key-12345!@#$"

	encrypted, err := store.encrypt([]byte(plaintext))
	if err != nil {
		t.Fatalf("encrypt failed: %v", err)
	}

	// 确认密文与明文不同且包含 nonce
	if len(encrypted) <= 12 {
		t.Errorf("encrypted data too short (%d bytes), expected nonce+ciphertext", len(encrypted))
	}

	decrypted, err := store.decrypt(encrypted)
	if err != nil {
		t.Fatalf("decrypt failed: %v", err)
	}

	if string(decrypted) != plaintext {
		t.Errorf("decrypted = %q, want %q", string(decrypted), plaintext)
	}
}

// TestEncryptedFileStore_CorruptFile 解密失败必须明确报错。
//
// 不能返回空结果：那会把「密钥漂移/文件损坏」在上层伪装成「还没有凭据」，
// 静默进入重新授权。也不能把密文当明文解析——等于凭据静默全丢且无痕迹。
// 坏文件备份为 .corrupt 保留诊断线索。
func TestEncryptedFileStore_CorruptFile(t *testing.T) {
	tmpDir := t.TempDir()
	store := &encryptedFileStore{
		path: filepath.Join(tmpDir, "settings", ".credentials"),
	}

	// 写入未加密的明文文件模拟损坏场景
	dir := filepath.Dir(store.path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	if err := os.WriteFile(store.path, []byte("not-encrypted=some-value"), 0600); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	val, err := store.Get("not-encrypted")
	if err == nil {
		t.Fatalf("损坏文件应返回明确错误，实际返回空结果 %q", val)
	}
	if val != "" {
		t.Errorf("损坏文件不应返回任何值，得到 %q", val)
	}
	if _, statErr := os.Stat(store.path + ".corrupt"); statErr != nil {
		t.Errorf("corrupt file not backed up as .corrupt: %v", statErr)
	}
}

// TestEncryptedFileStore_KeyFileRandomAndPrivate 本机密钥是随机落盘文件，
// 不再由主机名派生；文件权限 0600。
func TestEncryptedFileStore_KeyFileRandomAndPrivate(t *testing.T) {
	tmpDir := t.TempDir()
	store := &encryptedFileStore{
		path: filepath.Join(tmpDir, "settings", ".credentials"),
	}

	if err := store.Set("k", "v"); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	keyPath := store.keyFilePath()
	info, err := os.Stat(keyPath)
	if err != nil {
		t.Fatalf("本机密钥文件未落盘: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0600 {
		t.Errorf("密钥文件权限 = %o, want 600", perm)
	}
	key, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatalf("读取密钥失败: %v", err)
	}
	if len(key) != 32 {
		t.Fatalf("密钥长度 = %d, want 32", len(key))
	}
	if string(key) == string(store.legacyKey()) {
		t.Error("密钥仍是主机名派生值——随机化未生效")
	}

	// 密文不得含明文
	raw, err := os.ReadFile(store.path)
	if err != nil {
		t.Fatalf("读取密文失败: %v", err)
	}
	if string(raw) == "k=v\n" {
		t.Error("凭据文件是明文")
	}
}

// TestEncryptedFileStore_MigratesLegacyCiphertext 旧密钥密文可读且被就地迁移。
func TestEncryptedFileStore_MigratesLegacyCiphertext(t *testing.T) {
	tmpDir := t.TempDir()
	store := &encryptedFileStore{
		path: filepath.Join(tmpDir, "settings", ".credentials"),
	}
	if err := os.MkdirAll(filepath.Dir(store.path), 0700); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}

	// 用旧派生密钥写一份存量密文（模拟升级前的落盘）
	legacyCipher, err := encryptWith(store.legacyKey(), []byte("openai=sk-legacy\n"))
	if err != nil {
		t.Fatalf("构造存量密文失败: %v", err)
	}
	if err := os.WriteFile(store.path, legacyCipher, 0600); err != nil {
		t.Fatalf("写入存量密文失败: %v", err)
	}

	val, err := store.Get("openai")
	if err != nil {
		t.Fatalf("存量密文应可读取: %v", err)
	}
	if val != "sk-legacy" {
		t.Fatalf("openai = %q, want sk-legacy", val)
	}

	// 已迁移：文件可用新随机密钥解开，旧密钥解开失败
	migrated, err := os.ReadFile(store.path)
	if err != nil {
		t.Fatalf("读取迁移后文件失败: %v", err)
	}
	key, err := store.localKey()
	if err != nil {
		t.Fatalf("读取本机密钥失败: %v", err)
	}
	if _, err := decryptWith(key, migrated); err != nil {
		t.Fatalf("迁移后文件无法用新密钥解密: %v", err)
	}
	if _, err := decryptWith(store.legacyKey(), migrated); err == nil {
		t.Error("迁移后文件仍可用旧派生密钥解开，迁移未生效")
	}
}

// TestEncryptedFileStore_SetRefusesUnreadableBaseline 读失败时拒绝写入，
// 避免「读不到旧内容 → 按空表重写 → 其它凭据被静默清空」。
func TestEncryptedFileStore_SetRefusesUnreadableBaseline(t *testing.T) {
	tmpDir := t.TempDir()
	store := &encryptedFileStore{
		path: filepath.Join(tmpDir, "settings", ".credentials"),
	}
	if err := os.MkdirAll(filepath.Dir(store.path), 0700); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	broken := []byte("not-a-valid-ciphertext")
	if err := os.WriteFile(store.path, broken, 0600); err != nil {
		t.Fatalf("写入损坏文件失败: %v", err)
	}

	err := store.Set("k", "v")
	if err == nil {
		t.Fatal("基线不可读时 Set 应失败")
	}
	after, readErr := os.ReadFile(store.path)
	if readErr != nil {
		t.Fatalf("读取文件失败: %v", readErr)
	}
	if string(after) != string(broken) {
		t.Error("Set 覆盖了不可读的凭据文件，其余凭据已丢失")
	}
}

// TestResolveAPIKey 验证 ResolveAPIKey 从 store 中查询 Key。
func TestResolveAPIKey(t *testing.T) {
	tmpDir := t.TempDir()
	store := &encryptedFileStore{
		path: filepath.Join(tmpDir, "settings", ".credentials"),
	}

	if err := store.Set("my_provider", "pk-my-key"); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	result := ResolveAPIKey(store, "my_provider")
	if result != "pk-my-key" {
		t.Errorf("ResolveAPIKey = %q, want %q", result, "pk-my-key")
	}

	// 不存在的 Key
	result = ResolveAPIKey(store, "unknown")
	if result != "" {
		t.Errorf("ResolveAPIKey unknown = %q, want empty", result)
	}

	// nil store
	result = ResolveAPIKey(nil, "anything")
	if result != "" {
		t.Errorf("ResolveAPIKey with nil store = %q, want empty", result)
	}
}
