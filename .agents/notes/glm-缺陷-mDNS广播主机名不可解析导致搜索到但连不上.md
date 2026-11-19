# 缺陷：mDNS 广播的主机名不存在，客户端搜得到却连不上

日期：2026-11-20
范围：`mindx`（daemon）

## 现象

iPhone 真机打开 App 后卡在「等待主机批准」，`device.handshake` 一直回 `pending`。
排查发现手机连的是另一台 Mac，但更深一层的问题是：**这台 iMac 的 mDNS 广播本身就是坏的**。

```
$ dns-sd -L "MindX@iMac" _mindx._tcp local
MindX@iMac._mindx._tcp.local. can be reached at iMac.local.:1314

$ nc -z iMac.local 1314
nc: getaddrinfo: nodename nor servname provided, or not known
```

广播指向 `iMac.local`，这个名字**在本机都解析不出来**。服务名能搜到、端口也对，
地址却是死的——客户端拿到 SRV 记录后去解析 target，直接失败。

## 根因：macOS 有两套主机名

```
$ hostname                      # os.Hostname()，uname 名
iMac
$ scutil --get LocalHostName    # Bonjour 名
RaydeiMac
```

只有后者有 mDNS A 记录：

```
$ dscacheutil -q host -a name iMac.local        → 无记录
$ dscacheutil -q host -a name RaydeiMac.local   → 127.0.0.1 / 192.168.31.216
```

`grandcat/zeroconf` 的 `Register` 在 `HostName` 留空时会用 `os.Hostname()` 拼
`"<uname名>.local."`（见库源码 server.go:47-56），于是广播出一个不存在的主机名。

叠加第二个坑：该库的 `handleQuestion` 只应答 `ServiceTypeName` / `ServiceName` /
`ServiceInstanceName` 三类查询（server.go:376-395），**没有 A/AAAA 分支**。
即使地址记录里有内容，别人单查主机名的 IP 也不会得到应答。所以必须让
SRV target 本身就是一个系统 Bonjour 能解析的名字。

## 修法

`internal/discovery/mdns.go`：

1. 新增 `LocalHostName()`：macOS 走 `scutil --get LocalHostName`，其他平台回退
   `os.Hostname()`（并剥掉可能的 `.local` 后缀，防拼成 `x.local.local`）。
2. `Start` 改用 `zeroconf.RegisterProxy`，显式传 HostName 与 IP 列表。
   必须显式传 IP：`RegisterProxy` 不做自动探测，`ips` 为空则广播里没有 A 记录。
3. 新增 `localIPStrings()`：收集所有非回环、非链路本地地址（多网卡都广播，
   否则走 Wi-Fi / 有线的另一半用户连不上）。

`internal/svc/handler_server.go`：`server.version` 的 `hostname` 同步改用
`discovery.LocalHostName()`。**两边必须同源**：iOS 端用这个值与 mDNS 实例名
`MindX@<hostname>` 做「是不是同一台机器」的比对（地址自愈判据），
取不同的源会导致自愈永远匹配失败。

平台拆分：`platform_darwin.go` / `platform_other.go` 提供 `isDarwin`，
让非 macOS 构建完全不包含 `scutil` 调用路径。

## 验证

修复后同一进程（端口 18138）的广播：

```
MindX-ExternalProbe._mindx._tcp.local. can be reached at RaydeiMac.local.:18138

$ dscacheutil -q host -a name RaydeiMac.local
ip_address: 192.168.31.216     ← 可解析
```

新增 `internal/discovery/hostname_test.go`（3 条）：

- `TestLocalHostNameResolvable`：广播用名必须能解析出地址（不断言具体名字，
  因机器而异）。注意必须带 `.local` 后缀查，裸名对 Bonjour 名不是有效查询。
- `TestLocalHostNameNoDoubleSuffix`：防拼出 `x.local.local`。
- `TestLocalIPStrings`：广播必须带真实地址，且不得含回环/链路本地。

## 已知测试限制

`TestStartAndDiscover` 会在沙箱内失败——它用 `zeroconf.Browse` 在同一进程里
自己发现自己，而该库的自浏览在本机收不到自己的组播。**这条在改动前同样失败**
（已用 `git stash` 回退验证过），属库/环境限制，不是本次回归。判断广播是否正常
要用外部 `dns-sd -L` 查，不要只看这条测试。

## 教训

1. **「能搜到」和「能连上」是两件事。** mDNS 只保证服务记录可见，SRV target
   能否解析是另一个环节。验收局域网发现必须真的 `nc` 一下地址，不能只看列表里
   出现了设备。
2. **macOS 的 `os.Hostname()` 不等于 Bonjour 名。** 凡是拼 `.local` 的地方都要
   用 `scutil --get LocalHostName`，这个坑在 Go 里尤其容易犯，因为
   `os.Hostname()` 看起来就是"主机名"。
3. **同一个身份值不要从两个源取。** `server.version` 与 mDNS 实例名必须同源，
   否则跨端比对必然错位，而且症状（地址自愈不生效）离根因很远。

## 生效方式

代码改动需要重新构建并重启 daemon 才生效：

```bash
cd mindx && make build && make install
# 然后重启 MindX 桌面端（或重启 daemon 进程）
```

重启后用 `dns-sd -L "MindX@<主机名>" _mindx._tcp local` 确认 target 变成
`<Bonjour名>.local.` 且能被 `nc` 连上。