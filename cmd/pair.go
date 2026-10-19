package cmd

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/DotNetAge/mindx/pkg/rpc"
	"github.com/spf13/cobra"
)

var pairCmd = &cobra.Command{
	Use:   "pair",
	Short: "生成一次性配对码，供其他设备连接本机",
	Long: `生成一次性配对码，供其他设备（另一台 Mac、或出门在外连回来）连接本机。

用法：
  mindx pair

然后在连接方的 MindX 里打开「连接主机」，填入本机地址与配对码，
校验通过即完成连接，此后重连不再需要本机确认。

配对码 5 分钟内有效、只能用一次，输错 5 次即作废。`,
	RunE: runPair,
}

func init() {
	rootCmd.AddCommand(pairCmd)
}

func runPair(cmd *cobra.Command, args []string) error {
	client, err := rpc.Dial(daemonAddr)
	if err != nil {
		return err
	}
	defer client.Close()

	raw, err := client.CallWithTimeout("device.pair.code", map[string]any{})
	if err != nil {
		return fmt.Errorf("获取配对码失败: %w", err)
	}
	var result struct {
		Code       string `json:"code"`
		ExpiresAt  string `json:"expires_at"`
		TTLSeconds int    `json:"ttl_seconds"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return fmt.Errorf("解析配对码响应失败: %w", err)
	}

	fmt.Printf("配对码：%s\n\n", result.Code)
	fmt.Printf("有效期约 %d 分钟（至 %s），只能用一次，输错 5 次即作废。\n\n",
		(result.TTLSeconds+59)/60, result.ExpiresAt)
	fmt.Println("在连接方的 MindX 里打开「连接主机」：")
	fmt.Printf("  · 同一家庭/办公网络：在设备列表选择这台机器（%s），输入配对码\n", pairHostHint())
	if ip := lanIPHint(); ip != "" {
		fmt.Printf("  · 地址直填（跨公网填公网地址）：ws://%s:1314/ws\n", ip)
	}
	fmt.Println("\n校验通过即完成连接，本机无需再确认。")
	return nil
}

// pairHostHint 本机名称（mDNS 广播命名，连接方设备列表里看到的同款）。
func pairHostHint() string {
	name, err := os.Hostname()
	if err != nil || name == "" {
		return "这台机器"
	}
	return "MindX@" + name
}

// lanIPHint 返回本机首个非环回 IPv4（供本机主人复制地址用，不出现在设备列表）。
func lanIPHint() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return ""
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if !ok {
				continue
			}
			ip4 := ipNet.IP.To4()
			if ip4 == nil || ip4.IsLoopback() || ip4.IsLinkLocalUnicast() {
				continue
			}
			return strings.TrimSpace(ip4.String())
		}
	}
	return ""
}
