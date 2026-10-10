package core

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// uuidV4Pattern RFC 4122 随机版：第三组首位 4、第四组首位 8/9/a/b。
var uuidV4Pattern = regexp.MustCompile(
	`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

// TestMarketClientUUIDFormat 验证设备身份为 UUIDv4 格式且随机不重复。
func TestMarketClientUUIDFormat(t *testing.T) {
	seen := make(map[string]bool)
	for range 100 {
		id, err := marketClientUUID()
		if err != nil {
			t.Fatalf("marketClientUUID() error = %v", err)
		}
		if !uuidV4Pattern.MatchString(id) {
			t.Fatalf("生成的身份 %q 不符合 UUIDv4 格式", id)
		}
		if seen[id] {
			t.Fatalf("生成的身份 %q 重复", id)
		}
		seen[id] = true
	}
}

// TestSetMarketAuthTokenKeychain 验证登录令牌只进密钥存储：Get 回读一致，
// mindx.json 文件内容不含 token 明文。
func TestSetMarketAuthTokenKeychain(t *testing.T) {
	app := newSourceTestApp(t)

	if err := app.mindxConfig.Save(); err != nil {
		t.Fatalf("基线落盘失败：%v", err)
	}

	if err := app.SetMarketAuthToken("aut_test_secret"); err != nil {
		t.Fatalf("SetMarketAuthToken 失败：%v", err)
	}
	if got := app.MarketAuthToken(); got != "aut_test_secret" {
		t.Errorf("MarketAuthToken() = %q, want 内存同步", got)
	}
	if got, err := app.credStore.Get("market:auth_token"); err != nil || got != "aut_test_secret" {
		t.Errorf("密钥存储回读 = %q, err = %v, want 一致", got, err)
	}

	raw, err := os.ReadFile(app.mindxConfig.filePath)
	if err != nil {
		t.Fatalf("读 mindx.json 失败：%v", err)
	}
	if strings.Contains(string(raw), "aut_test_secret") {
		t.Error("mindx.json 不应包含登录令牌明文")
	}
}
