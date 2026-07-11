# 编辑演示文稿

## 基于模板的工作流程

使用现有演示文稿作为模板时，按以下步骤操作：

1. **分析现有幻灯片**：
   ```bash
   python scripts/thumbnail.py template.pptx
   python -m markitdown template.pptx
   ```
   查看 `thumbnails.jpg` 了解布局，查看 markitdown 输出了解占位符文本。

2. **规划幻灯片映射**：为每个内容板块选择合适的模板幻灯片。

   ⚠️ **使用多样化的布局** —— 千篇一律的布局是演示文稿最常见的失败原因。不要总是默认使用基础标题+项目符号的组合。主动寻找以下布局：
   - 多列布局（双栏、三栏）
   - 图片+文字组合
   - 满幅图片配叠加文字
   - 引用或高亮幻灯片
   - 章节分隔页
   - 数据/数字高亮
   - 图标网格或图标+文字行

   **避免**：每张幻灯片都重复使用同一种文字密集型布局。

   将内容类型与布局风格相匹配（例如：要点 → 项目符号幻灯片，团队信息 → 多列布局，用户评价 → 引用幻灯片）。

3. **解包**：`python scripts/office/unpack.py template.pptx unpacked/`

4. **构建演示文稿**（自行完成，不要交给子代理）：
   - 删除不需要的幻灯片（从 `<p:sldIdLst>` 中移除）
   - 复制需要重复使用的幻灯片（`add_slide.py`）
   - 在 `<p:sldIdLst>` 中重新排列幻灯片顺序
   - **在进入第 5 步之前，完成所有结构性修改**

5. **编辑内容**：更新每个 `slide{N}.xml` 中的文本。
   **如果可用，这一步请使用子代理** —— 每张幻灯片都是独立的 XML 文件，子代理可以并行编辑。

6. **清理**：`python scripts/clean.py unpacked/`

7. **打包**：`python scripts/office/pack.py unpacked/ output.pptx --original template.pptx`

---

## 脚本

| 脚本 | 用途 |
|--------|---------|
| `unpack.py` | 解包并格式化打印 PPTX |
| `add_slide.py` | 复制幻灯片或从布局创建幻灯片 |
| `clean.py` | 删除孤立文件 |
| `pack.py` | 验证并重新打包 |
| `thumbnail.py` | 生成幻灯片缩略图网格 |

### unpack.py

```bash
python scripts/office/unpack.py input.pptx unpacked/
```

解包 PPTX，格式化打印 XML，转义智能引号。

### add_slide.py

```bash
python scripts/add_slide.py unpacked/ slide2.xml      # 复制幻灯片
python scripts/add_slide.py unpacked/ slideLayout2.xml # 从布局创建
```

输出 `<p:sldId>`，需要将其添加到 `<p:sldIdLst>` 中的目标位置。

### clean.py

```bash
python scripts/clean.py unpacked/
```

删除不在 `<p:sldIdLst>` 中的幻灯片、未被引用的媒体文件以及孤立的 rels 文件。

### pack.py

```bash
python scripts/office/pack.py unpacked/ output.pptx --original input.pptx
```

验证、修复、压缩 XML，重新编码智能引号。

### thumbnail.py

```bash
python scripts/thumbnail.py input.pptx [output_prefix] [--cols N]
```

生成 `thumbnails.jpg`，以幻灯片文件名作为标签。默认 3 列，每个网格最多 12 张。

**仅用于模板分析**（选择布局）。如需视觉质量检查，请使用 `soffice` + `pdftoppm` 生成全分辨率的单独幻灯片图片 —— 详见 SKILL.md。

---

## 幻灯片操作

幻灯片顺序在 `ppt/presentation.xml` → `<p:sldIdLst>` 中定义。

**重排**：调整 `<p:sldId>` 元素的顺序。

**删除**：移除 `<p:sldId>`，然后运行 `clean.py`。

**添加**：使用 `add_slide.py`。切勿手动复制幻灯片文件 —— 该脚本会自动处理手动复制容易遗漏的笔记引用、Content_Types.xml 和关系 ID。

---

## 编辑内容

**子代理：** 如果可用，请在完成第 4 步后使用子代理。每张幻灯片都是独立的 XML 文件，子代理可以并行编辑。在发给子代理的提示中，请包含：
- 需要编辑的幻灯片文件路径
- **"所有修改都使用 Edit 工具"**
- 下方的格式规则和常见陷阱

对每张幻灯片：
1. 读取幻灯片的 XML
2. 识别所有占位符内容 —— 文本、图片、图表、图标、说明文字
3. 用最终内容替换每个占位符

**使用 Edit 工具，不要用 sed 或 Python 脚本。** Edit 工具会强制明确指定替换内容和位置，可靠性更高。

### 格式规则

- **所有标题、副标题和行内标签都要加粗**：在 `<a:rPr>` 上使用 `b="1"`。包括：
  - 幻灯片标题
  - 幻灯片内的章节标题
  - 行内标签（例如：行首的"状态："、"描述："）
- **不要使用 unicode 项目符号（•）**：使用 `<a:buChar>` 或 `<a:buAutoNum>` 进行正确的列表格式化
- **项目符号一致性**：让项目符号从布局继承。只需指定 `<a:buChar>` 或 `<a:buNone>`。

---

## 常见陷阱

### 模板适配

当源内容的项目数量少于模板时：
- **彻底删除多余元素**（图片、形状、文本框），不要只清空文字
- 清空文字内容后，检查是否有残留的孤立视觉元素
- 运行视觉质量检查，发现数量不匹配的问题

替换文本的长度与原文不同时：
- **替换内容更短**：通常没问题
- **替换内容更长**：可能溢出或意外换行
- 修改文字后进行视觉质量检查
- 考虑截断或拆分内容，以适应模板的设计约束

**模板槽位 ≠ 源项目数量**：如果模板有 4 个团队成员，但源数据只有 3 个人，需要删除第 4 个成员的整个分组（图片+文本框），而不只是删除文字。

### 多项目内容

如果源数据有多个项目（编号列表、多个章节），为每个项目创建独立的 `<a:p>` 元素 —— **切勿将所有内容拼接成一个字符串**。

**❌ 错误** —— 所有项目挤在一个段落里：
```xml
<a:p>
  <a:r><a:rPr .../><a:t>Step 1: Do the first thing. Step 2: Do the second thing.</a:t></a:r>
</a:p>
```

**✅ 正确** —— 每个段落独立，标题加粗：
```xml
<a:p>
  <a:pPr algn="l"><a:lnSpc><a:spcPts val="3919"/></a:lnSpc></a:pPr>
  <a:r><a:rPr lang="en-US" sz="2799" b="1" .../><a:t>Step 1</a:t></a:r>
</a:p>
<a:p>
  <a:pPr algn="l"><a:lnSpc><a:spcPts val="3919"/></a:lnSpc></a:pPr>
  <a:r><a:rPr lang="en-US" sz="2799" .../><a:t>Do the first thing.</a:t></a:r>
</a:p>
<a:p>
  <a:pPr algn="l"><a:lnSpc><a:spcPts val="3919"/></a:lnSpc></a:pPr>
  <a:r><a:rPr lang="en-US" sz="2799" b="1" .../><a:t>Step 2</a:t></a:r>
</a:p>
<!-- 继续此模式 -->
```

从原始段落复制 `<a:pPr>` 以保留行间距。标题使用 `b="1"` 加粗。

### 智能引号

unpack/pack 会自动处理智能引号。但 Edit 工具会将智能引号转换为 ASCII。

**添加包含引号的新文本时，使用 XML 实体：**

```xml
<a:t>the &#x201C;Agreement&#x201D;</a:t>
```

| 字符 | 名称 | Unicode | XML 实体 |
|-----------|------|---------|------------|
| `"` | 左双引号 | U+201C | `&#x201C;` |
| `"` | 右双引号 | U+201D | `&#x201D;` |
| `'` | 左单引号 | U+2018 | `&#x2018;` |
| `'` | 右单引号 | U+2019 | `&#x2019;` |

### 其他

- **空白字符**：如果 `<a:t>` 中有前导或尾随空格，使用 `xml:space="preserve"`
- **XML 解析**：使用 `defusedxml.minidom`，不要用 `xml.etree.ElementTree`（会破坏命名空间）
