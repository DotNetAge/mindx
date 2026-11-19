package discovery

import (
	"net"
	"os"
	"runtime"
	"testing"
)

// TestLocalHostNameResolvable 锁住一条真踩过的坑：广播用的主机名必须能被解析。
//
// 背景：原实现让 zeroconf 用 os.Hostname() 拼 SRV target。macOS 上那是 uname 名
// （如 "iMac"），而 Bonjour 主机名是另一个（`scutil --get LocalHostName`，
// 如 "RaydeiMac"）。于是广播出去的地址是一个**不存在的主机名**：客户端能搜到
// 服务、拿到端口，却解析不出 IP——表现为「局域网发现了设备但连不上」。
//
// 这里不断言具体名字（因机器而异），只断言「这个名字必须能解析出地址」。
func TestLocalHostNameResolvable(t *testing.T) {
	name := LocalHostName()
	if name == "" || name == "unknown-host" {
		t.Fatalf("主机名不可为空，实际 %q", name)
	}

	// 非 macOS 平台没有 Bonjour 名与 uname 名之分，跳过解析校验
	if runtime.GOOS != "darwin" {
		t.Logf("非 darwin 平台（%s），主机名 %q，跳过 Bonjour 解析校验", runtime.GOOS, name)
		return
	}

	// macOS 上 os.Hostname 与 Bonjour 名可能不同，二者都要记录便于排障
	unameName, _ := os.Hostname()
	t.Logf("uname 名=%q  Bonjour 名=%q", unameName, name)

	// 必须带 .local 后缀查：裸名对 Bonjour 名不是有效查询
	fqdn := name + "." + Domain
	addrs, err := net.LookupHost(fqdn)
	if err != nil {
		t.Fatalf("Bonjour 主机名 %q 无法解析（广播出去的地址将连不上）：%v", fqdn, err)
	}
	if len(addrs) == 0 {
		t.Fatalf("主机名 %q 解析结果为空", fqdn)
	}
	t.Logf("主机名 %q 解析为 %v", fqdn, addrs)
}

// TestLocalHostNameNoDoubleSuffix 防止拼出 "x.local.local"。
func TestLocalHostNameNoDoubleSuffix(t *testing.T) {
	name := LocalHostName()
	for _, suffix := range []string{".local", "." + Domain} {
		if len(name) > len(suffix) && name[len(name)-len(suffix):] == suffix {
			t.Fatalf("主机名不应带 local 后缀（拼 SRV 时会重复），实际 %q", name)
		}
	}
}

// TestLocalIPStrings 广播必须带上本机真实地址，否则 A 记录为空。
func TestLocalIPStrings(t *testing.T) {
	ips := localIPStrings()
	if len(ips) == 0 {
		t.Fatal("本机非回环 IP 列表为空：广播将不含任何 A 记录，客户端无法连接")
	}
	for _, ip := range ips {
		parsed := net.ParseIP(ip)
		if parsed == nil {
			t.Fatalf("非法 IP: %q", ip)
		}
		if parsed.IsLoopback() || parsed.IsLinkLocalUnicast() {
			t.Fatalf("不应广播回环/链路本地地址: %q", ip)
		}
	}
	t.Logf("广播地址: %v", ips)
}
