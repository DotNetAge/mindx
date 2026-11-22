# gort 本地副本说明

- 上游：`github.com/DotNetAge/gort`
- go.mod 声明版本：`v0.1.10`
- 实际使用方式：根 `go.mod` 用 `replace github.com/DotNetAge/gort => ./third_party/gort` 指向本目录

## 为什么有这份说明

`replace` 会让构建跳过 go.sum 对 gort 的校验，直接用工作区里的代码。这本身不是错误做法，但必须满足两个条件，否则问题很难被发现：

1. 副本与上游 `v0.1.10` 的差异是可查的；
2. 谁、为什么、何时升级副本有明确流程。

## 差异核对方法

在干净的环境里取上游同版本，与本地副本比较：

```
git clone https://github.com/DotNetAge/gort /tmp/gort-upstream
cd /tmp/gort-upstream && git checkout v0.1.10
diff -r /tmp/gort-upstream/pkg third_party/gort/pkg
```

差异必须能对应到本仓库的提交说明；不接受「改了但不知道改了什么」。

## 升级流程

1. 在上游检出目标 tag，记录 commit hash；
2. 用上游该版本覆盖 `third_party/gort/`（保留本文件）；
3. 同步修改根 `go.mod` 的 `require` 版本号与 `replace` 注释里的版本说明；
4. 跑 `go build ./... && go test ./...`，确认 gateway/channel 相关行为无回归；
5. 提交信息写明「升级 gort 至 vX.Y.Z（上游 commit）」及本地补丁的去留。

## 当前已知本地改动

副本中的 `CHANGES.md` 描述了 Channel-Gateway 整合（`message` 类型并入 `channel`、GatewaySender 注入等）。这些改动是否已回流上游、以及是否超出 `v0.1.10` 的范围，需要按上面的差异核对方法逐项确认后再更新本节。
