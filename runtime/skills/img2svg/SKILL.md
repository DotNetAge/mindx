---
name: img2svg
description: 将位图/内嵌位图的 SVG（logo、图标）转为矢量 SVG。当用户要求"位图转矢量/矢量化、去锯齿、生成黑白/纯白版本图标、提取 SVG 内嵌图片重绘"时使用。脚本化流程：分析 → 遮罩 → potrace 描摹 → 包装 → IoU 验证，调用 scripts/ 下脚本即可，无需现场写代码。
metadata:
  name_zh: 位图转矢量 SVG
---

# img2svg：位图转矢量 SVG

## 何时使用

- 用户要求把位图（PNG/JPG/内嵌 base64 位图的 SVG）转成矢量 SVG
- 图标/logo 放大有锯齿，需要矢量重绘
- 需要 logo 的黑白、纯白等衍生版本

## 工具链安装

| 工具           | 安装方式                                                                                    | 说明                                                                                                                                                        |
| -------------- | ------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------- |
| potrace ≥1.16  | `brew install potrace`                                                                      | Apple Silicon 装在 `/opt/homebrew/bin/potrace`，Intel 在 `/usr/local/bin/potrace`；trace.sh 的 `POTRACE` 变量按实际路径修改（用 `command -v potrace` 定位） |
| Pillow + numpy | `pip3 install pillow numpy`（超时换清华镜像 `-i https://pypi.tuna.tsinghua.edu.cn/simple`） | analyze.py / make_mask.py / package.py 依赖                                                                                                                 |
| Playwright     | `pip3 install playwright`（同上镜像）+ `playwright install chromium`                        | verify.py 自动发现 Chromium（先读环境变量 `CHROMIUM_PATH`，再探测 macOS/Linux 标准缓存目录取最新版）；自动发现失败时用 `CHROMIUM_PATH` 指定可执行文件       |

## 标准流程（五步，全部调用脚本）

脚本位置：`<技能目录>/scripts/`（下文以 `$S` 代指）。依赖的临时文件放 `/tmp/img2svg-work/`。

```bash
S="<本技能目录>/scripts"   # 技能加载时已能确定自身安装路径，勿硬编码具体机器路径
mkdir -p /tmp/img2svg-work
```

### 1. 分析源图（禁止跳过）

```bash
python3 $S/analyze.py <源图> --preview /tmp/img2svg-work/preview.png
```

输出尺寸、alpha 取值档数（仅 0/255 = 无抗锯齿）、形状像素数、亮度中位（底色取向）。**用 Read 工具目视 preview.png 确认图形结构**，不要凭想象。

### 2. 生成遮罩 PBM

```bash
python3 $S/make_mask.py <源图> /tmp/img2svg-work/mask.pbm
```

内置对账（黑位数 == 形状像素数），不一致会非零退出。阈值默认 128，特殊需求加 `--threshold`。

### 3. potrace 描摹

```bash
bash $S/trace.sh /tmp/img2svg-work/mask.pbm /tmp/img2svg-work/trace.svg
```

参数已定（`-s -t 2 -O 0.2`）。输出 path 坐标 ×10，且外层带 `transform="translate(0,H) scale(0.1,-0.1)"`——**path 的 d 值离开这个变换就是错的**（症状：图形被压到角落、IoU 掉到 0.6 左右）。package.py 会原样保留；自行包装（如 clipPath 场景）必须把 transform 并到 path 的 `transform` 属性上，不能只抠 `d`。

### 4. 包装多色版本

```bash
python3 $S/package.py /tmp/img2svg-work/trace.svg <宽> <高> <输出前缀> --colors black,white
```

生成 `<前缀>-mono.svg`（黑）与 `<前缀>-white.svg`（白），黑白共用同一条轮廓。宽高取源图尺寸。

### 5. 验证（必须真实渲染，禁止目测下结论）

```bash
python3 $S/verify.py <源图> 输出的矢量.svg ...
```

**适用范围：单色填充版**（黑/白纯色 path）。Playwright 渲染后与源图 alpha 形状逐像素比对，**IoU ≥ 0.97 通过**（渲染像素略少于真值属正常，是边缘平滑削减毛刺）；自动按 fill 色选判定方式（黑形白底 / 白形深底）。最后用 Read 目视截图复查边缘平滑度。

**原色版（clipPath + 内嵌位图结构）verify.py 不适用**（无 fill 可判定），现场写验证脚本，双指标：形状 IoU ≥ 0.97 + 非透明像素平均 RGB 差 ≈ 0（位图本体嵌入，颜色应零损失）。验证 HTML 必须用 `page.goto('file://...')` 直开，`set_content()` 会破图（见坑 3）。

## 已验证的坑（全部实战踩过）

1. **PBM 反相**：PIL 直接写 PBM 位序与预期不符，症状是 potrace 只描出 1 个大外框。make_mask.py 已用手工 P4 打包 + 内置对账规避。
2. **1 条轮廓 ≠ 错误**：相连图形（如 M+X 相触）本就是 1 个连通域。轮廓数异常时先还原 PBM 目视、逐行对账，再怀疑别的。
3. **file:// 限制**：凡 `set_content()` 注入的页面（origin about:blank）禁止加载 file:// 资源，img 全部破图（症状：截图空白、IoU≈0）。验证 HTML 一律落盘后 `page.goto('file://...')` 直开——自研脚本与现场验证脚本都适用。
4. **"破图"≠描摹错误**：先 XML 校验（`ET.parse`）与渲染方式排查，最后才怀疑 potrace。
5. **pt 单位**：potrace 输出 width/height 是 pt（1pt≈1.333px），package.py 已按 px 重写并对齐 viewBox。
6. **丢 transform**（原色版实踩）：包装 clipPath 时只抠 `d` 不带 `transform="translate(0,H) scale(0.1,-0.1)"`，症状是图形压到右下角、IoU≈0.63（右下对齐子集）。修正：transform 并到 path 属性上，复验即恢复 0.99+。

## 原色矢量版（保留原图颜色的 SVG）

场景：用户要"原图转 SVG"，不是黑/白单色。做法：**矢量轮廓裁剪 + 内嵌原色位图**——边缘由矢量曲线决定（平滑），颜色 100% 来自原图（注意：brew 无 vtracer formula，真正的彩色矢量化工具链不通，勿绕路安装）。

1. 取最高分辨率源：源图若是低清位图、同图形另有高分辨率版本（如同源 SVG 内嵌 base64），先提取高清版做源（提取：正则抓 `base64,` 后内容 b64decode）。同源确认：两版缩放比对 IoU ≥ 0.97
2. 遮罩 + 描摹：标准流程第 2、3 步（make_mask.py + trace.sh）
3. 包装（现场写小脚本）：`<clipPath id="shape"><path transform="<potrace 变换>" d="<d>"/></clipPath>` + `<image width="W" height="H" clip-path="url(#shape)" xlink:href="data:image/png;base64,..."/>`，viewBox 取源图尺寸。transform 必须跟随 path（见步骤 3 警示）；clipPath 内不要套 `<g>`（浏览器支持不稳），用 path 的 transform 属性
4. 验证：双指标（见步骤 5 原色版说明）

## 衍生版本速查

| 版本          | 做法                                                                                                      |
| ------------- | --------------------------------------------------------------------------------------------------------- |
| 矢量黑白/纯白 | 标准五步流程（本技能脚本）                                                                                |
| 原色矢量版    | 上节四步（clipPath + 内嵌位图）                                                                           |
| 灰度版        | RGB 亮度灰度化（R=G=B=L），alpha 原样保留，包回原 SVG 结构（现场写小脚本，参照 analyze.py 的 load_image） |
| 纯白位图版    | 所有像素涂白、alpha 原样保留（背景本透明时）                                                              |
| 硬边位图版    | alpha 阈值二值化（仅 0/255），纯色填充                                                                    |

## 源图选择原则

同一图形存在多分辨率版本时，**用最高分辨率版本做描摹源**（遮罩边缘精度决定描摹质量）；先用 analyze/缩放比对确认多版本同源（IoU ≥0.97），输出 viewBox 按需求尺寸包装。
