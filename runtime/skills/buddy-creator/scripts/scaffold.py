#!/usr/bin/env python3
"""生成一个 Buddy 组件骨架目录（不做美术，只出目录、清单与说明）。

用法：
  python3 scaffold.py --id my-pet --name 我的宠物 --renderer sprite --out ./my-pet
  python3 scaffold.py --help

输出：stdout 一份 JSON（生成的路径清单），stderr 一行结论。
退出码：0 成功；1 目标已存在或参数非法。
"""

from __future__ import annotations

import argparse
import json
import re
import sys
from pathlib import Path

ID_RE = re.compile(r"^[a-z0-9][a-z0-9-]{1,31}$")

MANIFESTS = {
    "sprite": {
        "renderer": "sprite",
        "model": "atlas.webp",
        "frames": {"count": 4, "cols": 4, "fps": 10},
        "viewport": {"width": 96, "height": 96, "anchor": "bottom-center"},
    },
    "gltf-clip": {
        "renderer": "gltf-clip",
        "model": "model.glb",
        "clips": {"idle": "Idle"},
        "viewport": {"width": 107, "height": 107, "anchor": "bottom-center"},
    },
    "custom": {
        "renderer": "custom",
        "model": "preview.png",
        "runtime": "runtime.js",
        "viewport": {"width": 107, "height": 107, "anchor": "bottom-center"},
    },
}

RUNTIME_STUB = """// custom 渲染器入口：跑在沙箱页面内，无 node、无网络。
// 只实现需要的动作，其余由壳兜底。
export function createBuddy(host) {
  const { container, assets } = host
  return {
    async mount() {
      // 在这里建 canvas / 载入资源，资源用 `${assets}xxx.png`
    },
    notify(kind) {},
    question() {},
    play() {},
    jump() {},
    busy(on) {},
    drag(on) {},
    lookat(x, y) {},
    dispose() {},
  }
}
"""

README = """# {name}（{id}）

渲染器：{renderer}

## 待补

{checklist}

## 校验

```bash
python3 <技能目录>/scripts/verify_buddy.py .
```

## 本地试装

```bash
cp -R . ~/.mindx/buddies/{id}
```
"""

CHECKLIST = {
    "sprite": "- [ ] 放入图集 `atlas.webp`（透明底，按 `frames.cols` 等分）\n- [ ] 在 `buddy.json` 的 `clips` 里声明帧区间，如 `\"idle\": \"0-1\"`",
    "gltf-clip": "- [ ] 放入 `model.glb`（内嵌贴图，含 `Idle` 动画 clip）\n- [ ] `clips` 逐项对齐 GLB 内真实 clip 名（大小写敏感）",
    "custom": "- [ ] 实现 `runtime.js` 的 `createBuddy`\n- [ ] 放入 `preview.png` 供设置页缩略展示",
}


def main():
    ap = argparse.ArgumentParser(description="生成 Buddy 组件骨架")
    ap.add_argument("--id", required=True, help="kebab-case 组件 id，须与目录名一致")
    ap.add_argument("--name", required=True, help="展示名")
    ap.add_argument("--renderer", default="sprite", choices=sorted(MANIFESTS))
    ap.add_argument("--out", required=True, help="输出目录（不存在则创建）")
    ap.add_argument("--version", default="0.1.0")
    ap.add_argument("--author", default="")
    ap.add_argument("--description", default="")
    args = ap.parse_args()

    if not ID_RE.match(args.id):
        print(f"[ERROR] id 不合规：{args.id}", file=sys.stderr)
        return 1

    out = Path(args.out).expanduser()
    if out.exists() and any(out.iterdir()):
        print(f"[ERROR] 目标目录已存在且非空：{out}", file=sys.stderr)
        return 1
    out.mkdir(parents=True, exist_ok=True)

    manifest = {"id": args.id, "name": args.name, "version": args.version}
    if args.author:
        manifest["author"] = args.author
    if args.description:
        manifest["description"] = args.description
    manifest.update(MANIFESTS[args.renderer])

    files = []
    mf = out / "buddy.json"
    mf.write_text(json.dumps(manifest, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    files.append(str(mf))

    rd = out / "README.md"
    rd.write_text(
        README.format(name=args.name, id=args.id, renderer=args.renderer,
                      checklist=CHECKLIST[args.renderer]),
        encoding="utf-8",
    )
    files.append(str(rd))

    if args.renderer == "custom":
        rt = out / "runtime.js"
        rt.write_text(RUNTIME_STUB, encoding="utf-8")
        files.append(str(rt))

    print(json.dumps({"dir": str(out), "renderer": args.renderer, "files": files},
                     ensure_ascii=False, indent=2))
    print(f"[OK] 已生成 {out}（{args.renderer}）", file=sys.stderr)
    return 0


if __name__ == "__main__":
    sys.exit(main())
