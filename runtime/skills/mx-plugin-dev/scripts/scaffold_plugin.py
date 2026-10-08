#!/usr/bin/env python3
"""mx-work 插件脚手架：按场景生成符合 demo 范式的插件骨架并自动注册。

Agent 生成插件的最高频路径：选场景 → 生成 → 按需修改，不手写样板。

用法（在 mindx-work 仓库根目录执行，路径不对时用 --target 指定）：
    python3 scaffold_plugin.py my-plugin                          # basic：页面 + 侧栏 + store
    python3 scaffold_plugin.py my-plugin --with detail            # 叠加：点侧栏自动打开 Detail
    python3 scaffold_plugin.py my-plugin --with settings services # 场景可任意叠加

场景（全部来自 demo 插件实证范式，细节读 references/examples.md）：
    basic     页面（Content + Sidebar + Pinia store），一切场景的地基
    detail    Detail tab 注册 + 页面挂载时自动打开（区间联动）
    settings  配置项 + 通用配置行（壳自带"通用"页）+ 自定义配置页
    overlay   modal（互斥）与 banner（可堆叠）浮层，页面按钮触发
    services  store 以服务形式 provide（跨插件共享；延迟外壳规避装配期 Pinia 未安装）

自动注册（锚点不匹配时跳过并提示手工步骤，绝不破坏既有文件）：
    1. plugins/package.json 的 exports 增加 "./<name>" 子路径
    2. plugins/src/index.ts 追加 re-export
    3. app/src/main.ts 的 createApp([...]) 追加插件

依赖：仅 Python 3 标准库。
"""

import argparse
import json
import re
import sys
from pathlib import Path

SCENARIOS = ("detail", "settings", "overlay", "services")

# ---------- 名称推导：kebab-case → camel/Pascal ----------


def to_parts(name: str) -> list[str]:
    parts = [p for p in re.split(r"[-_\s]+", name.strip()) if p]
    if not parts or not all(re.fullmatch(r"[A-Za-z0-9]+", p) for p in parts):
        raise SystemExit(f"插件名非法：{name!r}（需为 kebab-case，如 my-plugin）")
    return parts


def main() -> None:
    parser = argparse.ArgumentParser(description="按场景生成 mindx-work 插件骨架")
    parser.add_argument("name", help="插件名，kebab-case，如 my-plugin")
    parser.add_argument("--target", default="plugins/src", help="插件模块目录（默认 plugins/src）")
    parser.add_argument(
        "--with",
        dest="with_scenarios",
        nargs="+",
        choices=SCENARIOS,
        default=[],
        metavar="SCENARIO",
        help=f"叠加场景：{' / '.join(SCENARIOS)}（可多个；缺省为 basic）",
    )
    args = parser.parse_args()

    parts = to_parts(args.name)
    kebab = "-".join(parts).lower()
    camel = parts[0].lower() + "".join(p.capitalize() for p in parts[1:])
    pascal = "".join(p.capitalize() for p in parts)
    # 插件函数名：对齐 demo 范式（demoPlugin）；名内已含 plugin 时不再重复后缀
    plugin_fn = camel if camel.lower().endswith("plugin") else f"{camel}Plugin"
    scenarios = set(args.with_scenarios)

    target = Path(args.target).resolve() / kebab
    if target.exists():
        raise SystemExit(f"目标目录已存在，拒绝覆盖：{target}")
    plugins_pkg = target.parent.parent          # plugins/
    app_dir = plugins_pkg.parent / "app"        # app/

    # ---------- 场景代码片段（全部对齐 demo 插件真实范式） ----------

    # store：基础条目列表；settings 追加两个可写配置项
    store_extra = ""
    if "settings" in scenarios:
        store_extra = f"""  // 配置项：可写状态，设置行经 v-model 读写（配置读写范式读 references/examples.md）
  const limit = ref(10)
  const volume = ref(50)
"""
        store_return_extra = ", limit, volume"
    else:
        store_return_extra = ""

    # services 场景三件套：装配期 Pinia 尚未安装（插件函数体在 createApp 时执行，
    # createPinia 在 mountVueApp 内才装）——store 经延迟外壳按需创建；store 内取壳
    # 走 bind/theShell（shell.services.use 是内核方法，无 inject 依赖）。
    store_shell_import = ""
    store_shell_trio = ""
    store_service_shell = ""
    if "services" in scenarios:
        store_shell_import = "import type { VueAppShell } from '@mindx-work/ui-shell-vue'\n"
        store_shell_trio = f"""
// ── 壳引用绑定（装配期一次；store 需要壳能力时经 theShell() 取用）────────────

let shellRef: VueAppShell | null = null

/** 插件函数体内绑定 AppShell 本体（装配期 Pinia 尚未安装，禁止在此创建 store） */
export function bind{pascal}Shell(shell: VueAppShell): void {{
  shellRef = shell
}}

/** 插件内非组件代码取壳（组件内请用 useShell()） */
export function theShell(): VueAppShell {{
  if (!shellRef) throw new Error('{kebab} 壳未绑定：插件装配缺失')
  return shellRef
}}
"""
        store_service_shell = f"""
// ── 服务外壳（装配期 Pinia 尚未安装：getter 延迟到消费方首次解引用才创建 store）──

export type {pascal}Store = ReturnType<typeof use{pascal}Store>

export interface {pascal}Service {{
  readonly store: {pascal}Store
}}

export function create{pascal}Service(): {pascal}Service {{
  return {{
    get store() {{
      return use{pascal}Store()
    }},
  }}
}}
"""

    store_ts = f"""/** {kebab} 插件内部状态：一律 Pinia（界面层选型定稿） */

import {{ computed, ref }} from 'vue'
import {{ defineStore }} from 'pinia'
{store_shell_import}
interface Item {{
  id: number
  title: string
}}
{store_shell_trio}
export const use{pascal}Store = defineStore('{kebab}-store', () => {{
  const items = ref<Item[]>([])
{store_extra}  let nextId = 1

  const add = (title: string) => {{
    const trimmed = title.trim()
    if (trimmed) items.value.push({{ id: nextId++, title: trimmed }})
  }}
  const remove = (id: number) => {{
    items.value = items.value.filter((item) => item.id !== id)
  }}
  const count = computed(() => items.value.length)

  return {{ items{store_return_extra}, add, remove, count }}
}})
{store_service_shell}"""

    # HomePage script：detail 加 onMounted 自动打开；overlay 加浮层触发函数
    home_import_vue = "import { onMounted, ref } from 'vue'" if "detail" in scenarios else "import { ref } from 'vue'"
    home_import_shell = (
        "import { MxIcon, useShell } from '@mindx-work/ui-shell-vue'"
        if {"detail", "overlay"} & scenarios
        else "import { MxIcon } from '@mindx-work/ui-shell-vue'"
    )

    # shell 声明只出现一次（detail / overlay 任一场景需要）
    home_shell_decl = "\nconst shell = useShell()\n" if {"detail", "overlay"} & scenarios else ""

    home_script_detail = ""
    if "detail" in scenarios:
        home_script_detail = f"""
// Content 条目是 v-if 渲染：本页被 Sidebar 行选中挂载时自动打开 Detail（区间联动）
onMounted(() => {{
  shell.Detail.show('{kebab}-detail')
}})
"""

    home_script_overlay = ""
    if "overlay" in scenarios:
        home_script_overlay = f"""
const MODAL_ID = '{kebab}-modal'

// modal 互斥：同 id 先移除再添加（重复 add 抛错）；关闭 = 移除条目
const openModal = () => {{
  if (shell.Overlay.has(MODAL_ID)) shell.Overlay.remove(MODAL_ID)
  shell.Overlay.add({{ id: MODAL_ID, kind: 'modal', component: ConfirmModal }})
}}

// banner 可堆叠：自增序列号区分 id
let bannerSeq = 0
const pushBanner = () => {{
  bannerSeq += 1
  shell.Overlay.add({{ id: `{kebab}-banner-${{bannerSeq}}`, kind: 'banner', component: NoticeBanner }})
}}
"""
        overlay_imports = (
            "import ConfirmModal from '../overlays/ConfirmModal.vue'\n"
            "import NoticeBanner from '../overlays/NoticeBanner.vue'\n"
        )
    else:
        overlay_imports = ""

    home_template_overlay = ""
    if "overlay" in scenarios:
        home_template_overlay = """
    <div :class="$style.actions">
      <button type="button" class="mx-btn" @click="openModal">弹出对话框</button>
      <button type="button" class="mx-btn" @click="pushBanner">顶部通知</button>
    </div>
"""

    home_vue = f"""<script setup lang="ts">
/** {kebab} 主页：插件内部状态用 Pinia 持有，与壳机制状态互不掺和。 */
{home_import_vue}
{home_import_shell}
{overlay_imports}import {{ use{pascal}Store }} from '../store'
{home_shell_decl}{home_script_detail}{home_script_overlay}
const store = use{pascal}Store()
const draft = ref('')

const submit = () => {{
  store.add(draft.value)
  draft.value = ''
}}
</script>

<template>
  <div :class="$style.page">
    <h1 :class="$style.title">{kebab}</h1>
{home_template_overlay}
    <div :class="$style.composer">
      <input v-model="draft" type="text" class="mx-input" placeholder="添加条目…" @keydown.enter="submit" />
      <button type="button" class="mx-btn mx-btn--primary" @click="submit">添加</button>
    </div>
    <ul :class="$style.list">
      <li v-for="item in store.items" :key="item.id" :class="$style.item">
        <span :class="$style.itemTitle">{{{{ item.title }}}}</span>
        <button type="button" class="mx-icon-btn" aria-label="删除条目" @click="store.remove(item.id)">
          <MxIcon name="lucide:trash-2" :size="16" />
        </button>
      </li>
    </ul>
    <p :class="$style.count">共 {{{{ store.count }}}} 项</p>
  </div>
</template>

<style module>
.page {{
  padding: var(--mx-space-6);
  display: flex;
  flex-direction: column;
  gap: var(--mx-space-4);
}}

.title {{
  font: var(--mx-font-title);
  color: var(--mx-text);
  margin: 0;
}}

.actions {{
  display: flex;
  flex-wrap: wrap;
  gap: var(--mx-space-2);
}}

.composer {{
  display: flex;
  gap: var(--mx-space-2);
}}

.composer :global(.mx-input) {{
  flex: 1;
}}

.list {{
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: var(--mx-space-1);
}}

.item {{
  display: flex;
  align-items: center;
  gap: var(--mx-space-2);
  padding: var(--mx-space-1) var(--mx-space-2);
  border-radius: var(--mx-radius-control);
}}

.itemTitle {{
  flex: 1;
  font: var(--mx-font-body);
  color: var(--mx-text);
}}

.count {{
  font: var(--mx-font-caption);
  color: var(--mx-text-tertiary);
  margin: 0;
}}
</style>
"""

    # ---------- 场景附加文件 ----------

    extra_files: dict[Path, str] = {}

    row_css = """
.row {
  display: flex;
  align-items: center;
  gap: var(--mx-space-2);
  padding: 16px 0;
}

.rowText {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: var(--mx-space-1);
  padding-right: 48px;
}

.title {
  font: var(--mx-font-body);
  color: var(--mx-text);
}

.desc {
  font: var(--mx-font-caption);
  color: var(--mx-text-tertiary);
}

.input {
  width: 96px;
}
"""

    if "settings" in scenarios:
        extra_files[target / "prefs" / "GeneralRow.vue"] = f"""<script setup lang="ts">
/** 通用配置行（落壳自带"通用"页）：v-model 直读直写 store 配置项。 */
import {{ use{pascal}Store }} from '../store'

const store = use{pascal}Store()
</script>

<template>
  <div :class="$style.row">
    <div :class="$style.rowText">
      <span :class="$style.title">数量上限</span>
      <span :class="$style.desc">通用页配置行：不传 page 即归入壳自带"通用"页</span>
    </div>
    <input v-model.number="store.limit" type="number" class="mx-input" :class="$style.input" />
  </div>
</template>

<style module>
{row_css}</style>
"""
        extra_files[target / "prefs" / "PageRow.vue"] = f"""<script setup lang="ts">
/** 自定义配置页里的行：同 store 第二个配置项，跨插件消费走 services。 */
import {{ use{pascal}Store }} from '../store'

const store = use{pascal}Store()
</script>

<template>
  <div :class="$style.row">
    <div :class="$style.rowText">
      <span :class="$style.title">音量</span>
      <span :class="$style.desc">自定义配置页"{pascal}设置"中的行</span>
    </div>
    <input v-model.number="store.volume" type="number" class="mx-input" :class="$style.input" />
  </div>
</template>

<style module>
{row_css}</style>
"""

    if "detail" in scenarios:
        extra_files[target / "DetailPanel.vue"] = f"""<script setup lang="ts">
/** {kebab} 详情面板（Detail tab 内容）。 */
</script>

<template>
  <div :class="$style.panel">
    <p :class="$style.hint">详情轨道内容：由主页挂载时经 shell.Detail.show('{kebab}-detail') 打开。</p>
  </div>
</template>

<style module>
.panel {{
  padding: var(--mx-space-4);
}}

.hint {{
  font: var(--mx-font-caption);
  color: var(--mx-text-tertiary);
  margin: 0;
}}
</style>
"""

    if "overlay" in scenarios:
        extra_files[target / "overlays" / "ConfirmModal.vue"] = f"""<script setup lang="ts">
/** modal 浮层：关闭 = 移除条目（has 守卫防重复触发）。 */
import {{ MxIcon, useShell }} from '@mindx-work/ui-shell-vue'

const shell = useShell()
const MODAL_ID = '{kebab}-modal'

const close = () => {{
  if (shell.Overlay.has(MODAL_ID)) shell.Overlay.remove(MODAL_ID)
}}
</script>

<template>
  <div :class="$style.modal">
    <div :class="$style.head">
      <MxIcon name="lucide:circle-alert" :size="16" />
      <h2 :class="$style.heading">确认操作</h2>
    </div>
    <p :class="$style.body">同一时刻至多一个 modal；popover / sheet 归插件局部渲染，不进 Overlay。</p>
    <div :class="$style.actions">
      <button type="button" class="mx-btn" @click="close">取消</button>
      <button type="button" class="mx-btn mx-btn--primary" @click="close">确认</button>
    </div>
  </div>
</template>

<style module>
/* 卡片壳（背景/圆角/min-width/padding/阴影）由壳的 modalCard 容器提供，
 * 条目组件只写内部排版。 */
.modal {{
  display: flex;
  flex-direction: column;
  gap: var(--mx-space-3);
}}

.head {{
  display: flex;
  align-items: center;
  gap: var(--mx-space-2);
}}

.heading {{
  font: var(--mx-font-heading);
  color: var(--mx-text);
  margin: 0;
}}

.body {{
  font: var(--mx-font-body);
  color: var(--mx-text-secondary);
  margin: 0;
}}

.actions {{
  display: flex;
  justify-content: flex-end;
  gap: var(--mx-space-2);
}}
</style>
"""
        extra_files[target / "overlays" / "NoticeBanner.vue"] = f"""<script setup lang="ts">
/** banner 浮层：纯内容组件——壳不传 props，关闭钮由壳容器提供（关闭 = 移除条目）。 */
import {{ MxIcon }} from '@mindx-work/ui-shell-vue'
</script>

<template>
  <div :class="$style.banner">
    <MxIcon name="lucide:bell" :size="16" />
    <span>顶部通知</span>
  </div>
</template>

<style module>
.banner {{
  display: flex;
  align-items: center;
  gap: var(--mx-space-2);
  font: var(--mx-font-body);
  color: var(--mx-text);
}}
</style>
"""

    # ---------- index.ts：基础注册 + 场景片段 ----------

    index_imports = [
        "import type { VuePlugin } from '@mindx-work/ui-shell-vue'",
        "import HomePage from './pages/HomePage.vue'",
    ]
    if "settings" in scenarios:
        index_imports.append("import GeneralRow from './prefs/GeneralRow.vue'")
        index_imports.append("import PageRow from './prefs/PageRow.vue'")
    if "detail" in scenarios:
        index_imports.append("import DetailPanel from './DetailPanel.vue'")
    # store 导入仅 services 场景需要（其余场景 index.ts 不直接消费 store，
    # HomePage / 行组件各自导入——避免生成未使用导入）
    if "services" in scenarios:
        index_imports.append(f"import {{ bind{pascal}Shell, create{pascal}Service }} from './store'")

    index_detail = ""
    if "detail" in scenarios:
        index_detail = f"""
  // Detail：tab 注册（HomePage 挂载时自动 show，区间联动）
  ctx.Detail.add({{
    id: '{kebab}-detail',
    title: '详情',
    icon: 'lucide:info',
    component: DetailPanel,
  }})
"""

    index_settings = ""
    if "settings" in scenarios:
        index_settings = f"""
  // Settings：通用配置行（不传 page 归入壳自带"通用"页）+ 自定义配置页
  ctx.Settings.row({{ id: 'pref-{kebab}-general', component: GeneralRow }})
  ctx.Settings.page({{
    id: '{kebab}-prefs',
    title: '{pascal}设置',
    icon: 'lucide:settings',
    component: PageRow,
  }})
  ctx.Settings.row({{ id: 'pref-{kebab}-page', page: '{kebab}-prefs', component: PageRow }})
"""

    index_bind = ""
    if "services" in scenarios:
        index_bind = f"""
  // 装配期捕获壳本体（store 顶部零 inject 依赖的关键；此时 Pinia 尚未安装，
  // 插件函数体内禁止 use{pascal}Store()——装配时序坑详见 references/examples.md）
  bind{pascal}Shell(ctx)
"""

    index_services = ""
    if "services" in scenarios:
        index_services = f"""
  // services：store 延迟外壳（getter 在消费方首次解引用时才创建 store，首次
  // 解引用须在挂载后；通道里流动的是 Pinia store 响应式本体，不是快照副本）
  ctx.services.provide('{kebab}.store', create{pascal}Service())
"""

    scenario_note = (
        f"叠加场景：{', '.join(sorted(scenarios))}" if scenarios else "basic：页面 + 侧栏 + store"
    )
    index_ts = f"""/**
 * {kebab} 插件：函数 + 可选清理函数（契约第 7 节）。
 * 骨架由 mx-plugin-dev/scripts/scaffold_plugin.py 生成（{scenario_note}），
 * 行 id 与 Content 条目一一对应（启动期校验）。
 */

{chr(10).join(index_imports)}

export const {plugin_fn}: VuePlugin = (ctx) => {{{index_bind}
  // Content：页面（条目 id 带插件前缀，避免跨插件撞 id）
  ctx.Content.add({{ id: '{kebab}-home', order: 100, title: '{kebab}', component: HomePage }})

  // Sidebar：节 + 行（行 id 必须对应 Content 条目 id，启动期校验）
  ctx.Sidebar.add({{
    id: '{kebab}-nav',
    order: 100,
    title: '{pascal}',
    icon: 'lucide:puzzle',
    component: HomePage,
    rows: [{{ id: '{kebab}-home', label: '{kebab}', icon: 'lucide:puzzle' }}],
  }})
{index_detail}{index_settings}{index_services}
  return () => {{}}
}}
"""

    # ---------- 写盘 ----------

    files: dict[Path, str] = {
        target / "index.ts": index_ts,
        target / "store.ts": store_ts,
        target / "pages" / "HomePage.vue": home_vue,
        **extra_files,
    }
    for path in files:
        path.parent.mkdir(parents=True, exist_ok=True)
    for path, content in files.items():
        path.write_text(content, encoding="utf-8")
    print(f"已生成插件模块：{target}（{scenario_note}）")
    for path in files:
        print(f"  {path.relative_to(target.parent.parent)}")

    manual: list[str] = []

    # 注册 1：plugins/package.json exports 子路径
    pkg_path = plugins_pkg / "package.json"
    if pkg_path.exists():
        pkg = json.loads(pkg_path.read_text(encoding="utf-8"))
        exports = pkg.setdefault("exports", {})
        if "." in exports and f"./{kebab}" not in exports:
            exports[f"./{kebab}"] = f"./src/{kebab}/index.ts"
            pkg_path.write_text(json.dumps(pkg, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
            print(f"已注册 exports：{pkg_path} → \"./{kebab}\"")
        elif f"./{kebab}" in exports:
            print(f"exports 已含 ./{kebab}，跳过")
    else:
        manual.append(f'在 plugins/package.json 的 exports 中加："./{kebab}": "./src/{kebab}/index.ts"')

    # 注册 2：plugins/src/index.ts re-export
    root_ts = plugins_pkg / "src" / "index.ts"
    export_line = f"export {{ default as {plugin_fn} }} from './{kebab}/index'"
    if root_ts.exists():
        text = root_ts.read_text(encoding="utf-8")
        if export_line in text:
            print(f"re-export 已存在，跳过：{root_ts}")
        else:
            root_ts.write_text(text.rstrip("\n") + f"\n{export_line}\n", encoding="utf-8")
            print(f"已追加 re-export：{root_ts}")
    else:
        manual.append(f"在 plugins/src/index.ts 加：{export_line}")

    # 注册 3：app/src/main.ts 装配清单
    main_ts = app_dir / "src" / "main.ts"
    if main_ts.exists():
        text = main_ts.read_text(encoding="utf-8")
        if plugin_fn in text:
            print(f"装配清单已含 {plugin_fn}，跳过")
        else:
            new_imports = f"import {{ {plugin_fn} }} from '@mindx-work/plugins/{kebab}'\n"
            # import 块末尾插入（最后一个顶层 import 之后）
            matches = list(re.finditer(r"^import .*?\n", text, flags=re.M))
            if matches:
                last = matches[-1]
                text = text[: last.end()] + new_imports + text[last.end() :]
            else:
                text = new_imports + text
            # createApp([demoPlugin]) / createApp([a, b]) 追加到数组末位
            new_text, n = re.subn(
                r"createApp\(\[([^\]]*)\]\)",
                lambda m: "createApp([" + m.group(1).rstrip() + f", {plugin_fn}])",
                text,
                count=1,
            )
            if n:
                main_ts.write_text(new_text, encoding="utf-8")
                print(f"已追加装配：{main_ts} → createApp([..., {plugin_fn}])")
            else:
                main_ts.write_text(text, encoding="utf-8")
                manual.append(f"在 app/src/main.ts 的 createApp([...]) 数组中追加 {plugin_fn}")
    else:
        manual.append(f"在 app/src/main.ts 的 createApp([...]) 数组中追加 {plugin_fn}")

    if manual:
        print("\n以下注册需手工完成（锚点不匹配，未改动文件）：")
        for step in manual:
            print(f"  - {step}")

    print("\n验证：pnpm typecheck（mindx-work 目录），再起 dev 断言渲染。")


if __name__ == "__main__":
    try:
        main()
    except SystemExit:
        raise
    except Exception as exc:  # 兜底：任何失败都不留半成品
        print(f"失败：{exc}", file=sys.stderr)
        sys.exit(1)
