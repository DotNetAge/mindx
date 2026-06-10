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

// TestEncryptedFileStore_DecryptInvalid 验证对无效密文解密的容错（不 panic）。
func TestEncryptedFileStore_DecryptInvalid(t *testing.T) {
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

	// readEncrypted 当解密失败时会返回原始数据（容错）
	val, err := store.Get("not-encrypted")
	if err != nil {
		t.Fatalf("Get on corrupt file failed: %v", err)
	}
	if val != "some-value" {
		t.Errorf("got %q, want %q", val, "some-value")
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
