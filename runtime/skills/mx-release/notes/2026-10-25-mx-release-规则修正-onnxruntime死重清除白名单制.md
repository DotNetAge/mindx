---
version: "1.5.1"
pub_date: 2026-10-25
sha: "770d3823c2ca26b87cd81509058882c2089b72d6e9d0462b1496a609b87af436"
modifier: "Ray"
modifier_id: "1"
---

## 2026-10-25 onnxruntime 死重清除：白名单清理制（glm）

- 触发：用户质问「onnxruntime 不是早随大重构剥离出 daemon 了吗，为什么还在？」——用户记忆正确。实证三连：①mindx 仓库 grep go.mod/*.go/workflows 零引用；②当前 build-mindx-bundle.mjs 已不下载 onnxruntime（只下 mindx 裸二进制，2.6.x runtime 内嵌），daemon-installer.ts 零引用；③electron/resources/mindx/ 躺着 2026-10-02（2.5.10 时代第一期实施）下载的两个 tgz（31.7M+5.3M=37M）。
- 进包机制：清理逻辑旧黑名单只匹配 `mindx-[\d.]+-` 前缀（onnxruntime 文件名漏匹配→永不清理）+ electron-builder extraResources 整目录拷贝 → 残留进 .app 且被签名（dist.bak CodeResources 实锤）。
- 修正：删两 tgz 残留；清理逻辑改白名单制（只留当前版本 mindx 归档 + version.json，其余 recursive 删）；SKILL 坑 13（含反向风险：新增下载产物必须同步改白名单，否则被自己误删）。第五轮构建已被停掉重启（原会话把残留打进包）。
- 教训：整目录 extraResources 是死文件积尘的放大器——目录类注入必须有白名单/清单制防线；「机制已剥离」不等于「历史产物已清除」，剥离后应立即清一次存量。
