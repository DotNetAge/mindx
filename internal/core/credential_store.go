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
	if v := os.Getenv(ref); v != "" {
		return v
	}
	if store != nil {
		if v, err := store.Get(ref); err == nil && v != "" {
			return v
		}
	}
	return ref
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

func (s *encryptedFileStore) encryptKey() []byte {
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

	plaintext, err := s.decrypt(data)
	if err != nil {
		return string(data), nil
	}
	return string(plaintext), nil
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
	key := s.encryptKey()
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

func (s *encryptedFileStore) decrypt(ciphertext []byte) ([]byte, error) {
	key := s.encryptKey()
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	if len(ciphertext) < aead.NonceSize() {
		return ciphertext, fmt.Errorf("data too short for AES-GCM")
	}

	nonce, ciphertext := ciphertext[:aead.NonceSize()], ciphertext[aead.NonceSize():]
	plaintext, err := aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}

	return plaintext, nil
}
