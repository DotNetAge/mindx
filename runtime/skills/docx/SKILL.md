---
name: docx
description: "当用户想要创建、读取、编辑或操作 Word 文档(.docx 文件)时使用此技能。触发条件包括:任何提到 'Word doc'、'word document'、'.docx' 的请求,或要求生成带有格式化功能(如目录、标题、页码或信头)的专业文档。也适用于从 .docx 文件中提取或重新组织内容、在文档中插入或替换图片、在 Word 文件中进行查找和替换、处理修订跟踪或批注,或将内容转换为精美的 Word 文档。如果用户要求以 Word 或 .docx 文件形式交付 '报告'、'备忘录'、'信件'、'模板' 或类似的可交付成果,请使用此技能。不适用于 PDF、电子表格、Google Docs 或与文档生成无关的通用编码任务。"
license: Proprietary. LICENSE.txt has complete terms
metadata:
  name_zh: Word 文档处理
  name_zh-tw: Word 文件處理
  description_zh: 创建、读取、编辑和操作 Word 文档文件,支持目录、分页、表格、图片、修订和批注等功能
  description_zh-tw: 建立、讀取、編輯和操作 Word 文件檔案,支援目錄、分頁、表格、圖片、修訂和批註等功能
---

# DOCX 创建、编辑和分析

## 概述

.docx 文件是包含 XML 文件的 ZIP 归档。

## 快速参考

| 任务          | 方法                                                |
| ------------- | --------------------------------------------------- |
| 读取/分析内容 | `pandoc` 或解包获取原始 XML                         |
| 创建新文档    | 使用 `docx-js` - 参见下方"创建新文档"               |
| 编辑现有文档  | 解包 → 编辑 XML → 重新打包 - 参见下方"编辑现有文档" |

### 将 .doc 转换为 .docx

旧版 `.doc` 文件在编辑前必须转换:

```bash
python scripts/office/soffice.py --headless --convert-to docx document.doc
```

### 读取内容

```bash
# 带修订跟踪的文本提取
pandoc --track-changes=all document.docx -o output.md

# 原始 XML 访问
python scripts/office/unpack.py document.docx unpacked/
```

### 转换为图片

```bash
python scripts/office/soffice.py --headless --convert-to pdf document.docx
pdftoppm -jpeg -r 150 document.pdf page
```

### 接受修订跟踪

生成接受所有修订跟踪的干净文档(需要 LibreOffice):

```bash
python scripts/accept_changes.py input.docx output.docx
```

---

## 创建新文档

使用 JavaScript 生成 .docx 文件,然后验证。安装:`npm install -g docx`

### 设置
```javascript
const { Document, Packer, Paragraph, TextRun, Table, TableRow, TableCell, ImageRun,
        Header, Footer, AlignmentType, PageOrientation, LevelFormat, ExternalHyperlink,
        InternalHyperlink, Bookmark, FootnoteReferenceRun, PositionalTab,
        PositionalTabAlignment, PositionalTabRelativeTo, PositionalTabLeader,
        TabStopType, TabStopPosition, Column, SectionType,
        TableOfContents, HeadingLevel, BorderStyle, WidthType, ShadingType,
        VerticalAlign, PageNumber, PageBreak } = require('docx');

const doc = new Document({ sections: [{ children: [/* 内容 */] }] });
Packer.toBuffer(doc).then(buffer => fs.writeFileSync("doc.docx", buffer));
```

### 验证
创建文件后进行验证。如果验证失败,解包、修复 XML 并重新打包。
```bash
python scripts/office/validate.py doc.docx
```

### 页面尺寸

```javascript
// 关键:docx-js 默认使用 A4,而非美国 Letter
// 始终明确设置页面尺寸以确保一致的结果
sections: [{
  properties: {
    page: {
      size: {
        width: 12240,   // 8.5 英寸,单位为 DXA
        height: 15840   // 11 英寸,单位为 DXA
      },
      margin: { top: 1440, right: 1440, bottom: 1440, left: 1440 } // 1 英寸边距
    }
  },
  children: [/* 内容 */]
}]
```

**常见页面尺寸(DXA 单位,1440 DXA = 1 英寸):**

| 纸张        | 宽度   | 高度   | 内容宽度(1" 边距) |
| ----------- | ------ | ------ | ----------------- |
| 美国 Letter | 12,240 | 15,840 | 9,360             |
| A4(默认)    | 11,906 | 16,838 | 9,026             |

**横向:** docx-js 在内部交换宽度/高度,所以传入纵向尺寸并让它处理交换:
```javascript
size: {
  width: 12240,   // 传入短边作为宽度
  height: 15840,  // 传入长边作为高度
  orientation: PageOrientation.LANDSCAPE  // docx-js 在 XML 中交换它们
},
// 内容宽度 = 15840 - 左边距 - 右边距(使用长边)
```

### 样式(覆盖内置标题)

使用 Arial 作为默认字体(通用支持)。标题保持黑色以提高可读性。

```javascript
const doc = new Document({
  styles: {
    default: { document: { run: { font: "Arial", size: 24 } } }, // 12pt 默认
    paragraphStyles: [
      // 重要:使用精确 ID 覆盖内置样式
      { id: "Heading1", name: "Heading 1", basedOn: "Normal", next: "Normal", quickFormat: true,
        run: { size: 32, bold: true, font: "Arial" },
        paragraph: { spacing: { before: 240, after: 240 }, outlineLevel: 0 } }, // TOC 需要 outlineLevel
      { id: "Heading2", name: "Heading 2", basedOn: "Normal", next: "Normal", quickFormat: true,
        run: { size: 28, bold: true, font: "Arial" },
        paragraph: { spacing: { before: 180, after: 180 }, outlineLevel: 1 } },
    ]
  },
  sections: [{
    children: [
      new Paragraph({ heading: HeadingLevel.HEADING_1, children: [new TextRun("标题")] }),
    ]
  }]
});
```

### 列表(永远不要使用 Unicode 项目符号)

```javascript
// ❌ 错误 - 永远不要手动插入项目符号字符
new Paragraph({ children: [new TextRun("• 项目")] })  // 错误
new Paragraph({ children: [new TextRun("\u2022 项目")] })  // 错误

// ✅ 正确 - 使用带 LevelFormat.BULLET 的编号配置
const doc = new Document({
  numbering: {
    config: [
      { reference: "bullets",
        levels: [{ level: 0, format: LevelFormat.BULLET, text: "•", alignment: AlignmentType.LEFT,
          style: { paragraph: { indent: { left: 720, hanging: 360 } } } }] },
      { reference: "numbers",
        levels: [{ level: 0, format: LevelFormat.DECIMAL, text: "%1.", alignment: AlignmentType.LEFT,
          style: { paragraph: { indent: { left: 720, hanging: 360 } } } }] },
    ]
  },
  sections: [{
    children: [
      new Paragraph({ numbering: { reference: "bullets", level: 0 },
        children: [new TextRun("项目符号项")] }),
      new Paragraph({ numbering: { reference: "numbers", level: 0 },
        children: [new TextRun("编号项")] }),
    ]
  }]
});

// ⚠️ 每个引用创建独立的编号
// 相同引用 = 继续(1,2,3 然后 4,5,6)
// 不同引用 = 重新开始(1,2,3 然后 1,2,3)
```

### 表格

**关键:表格需要双重宽度** - 同时设置表格上的 `columnWidths` 和每个单元格上的 `width`。缺少任一个,表格在某些平台上会渲染不正确。

```javascript
// 关键:始终设置表格宽度以确保一致渲染
// 关键:使用 ShadingType.CLEAR(不是 SOLID)以防止黑色背景
const border = { style: BorderStyle.SINGLE, size: 1, color: "CCCCCC" };
const borders = { top: border, bottom: border, left: border, right: border };

new Table({
  width: { size: 9360, type: WidthType.DXA }, // 始终使用 DXA(百分比在 Google Docs 中会出错)
  columnWidths: [4680, 4680], // 必须总和等于表格宽度(DXA:1440 = 1 英寸)
  rows: [
    new TableRow({
      children: [
        new TableCell({
          borders,
          width: { size: 4680, type: WidthType.DXA }, // 也在每个单元格上设置
          shading: { fill: "D5E8F0", type: ShadingType.CLEAR }, // CLEAR 不是 SOLID
          margins: { top: 80, bottom: 80, left: 120, right: 120 }, // 单元格内边距(内部,不添加到宽度)
          children: [new Paragraph({ children: [new TextRun("单元格")] })]
        })
      ]
    })
  ]
})
```

**表格宽度计算:**

始终使用 `WidthType.DXA` — `WidthType.PERCENTAGE` 在 Google Docs 中会出错。

```javascript
// 表格宽度 = columnWidths 之和 = 内容宽度
// 美国 Letter 1" 边距:12240 - 2880 = 9360 DXA
width: { size: 9360, type: WidthType.DXA },
columnWidths: [7000, 2360]  // 必须总和等于表格宽度
```

**宽度规则:**
- **始终使用 `WidthType.DXA`** — 永远不要使用 `WidthType.PERCENTAGE`(与 Google Docs 不兼容)
- 表格宽度必须等于 `columnWidths` 之和
- 单元格 `width` 必须匹配对应的 `columnWidth`
- 单元格 `margins` 是内部内边距 - 它们减少内容区域,不添加到单元格宽度
- 对于全宽表格:使用内容宽度(页面宽度减去左右边距)

### 图片

```javascript
// 关键:type 参数是必需的
new Paragraph({
  children: [new ImageRun({
    type: "png", // 必需:png、jpg、jpeg、gif、bmp、svg
    data: fs.readFileSync("image.png"),
    transformation: { width: 200, height: 150 },
    altText: { title: "标题", description: "描述", name: "名称" } // 三个都必需
  })]
})
```

### 分页符

```javascript
// 关键:PageBreak 必须在 Paragraph 内
new Paragraph({ children: [new PageBreak()] })

// 或使用 pageBreakBefore
new Paragraph({ pageBreakBefore: true, children: [new TextRun("新页面")] })
```

### 超链接

```javascript
// 外部链接
new Paragraph({
  children: [new ExternalHyperlink({
    children: [new TextRun({ text: "点击这里", style: "Hyperlink" })],
    link: "https://example.com",
  })]
})

// 内部链接(书签 + 引用)
// 1. 在目标位置创建书签
new Paragraph({ heading: HeadingLevel.HEADING_1, children: [
  new Bookmark({ id: "chapter1", children: [new TextRun("第 1 章")] }),
]})
// 2. 链接到它
new Paragraph({ children: [new InternalHyperlink({
  children: [new TextRun({ text: "参见第 1 章", style: "Hyperlink" })],
  anchor: "chapter1",
})]})
```

### 脚注

```javascript
const doc = new Document({
  footnotes: {
    1: { children: [new Paragraph("来源:2024 年年报")] },
    2: { children: [new Paragraph("方法论详见附录")] },
  },
  sections: [{
    children: [new Paragraph({
      children: [
        new TextRun("收入增长 15%"),
        new FootnoteReferenceRun(1),
        new TextRun(" 使用调整后指标"),
        new FootnoteReferenceRun(2),
      ],
    })]
  }]
});
```

### 制表位

```javascript
// 在同一行右对齐文本(例如,日期与标题相对)
new Paragraph({
  children: [
    new TextRun("公司名称"),
    new TextRun("\t2025 年 1 月"),
  ],
  tabStops: [{ type: TabStopType.RIGHT, position: TabStopPosition.MAX }],
})

// 点线引导符(例如,目录样式)
new Paragraph({
  children: [
    new TextRun("引言"),
    new TextRun({ children: [
      new PositionalTab({
        alignment: PositionalTabAlignment.RIGHT,
        relativeTo: PositionalTabRelativeTo.MARGIN,
        leader: PositionalTabLeader.DOT,
      }),
      "3",
    ]}),
  ],
})
```

### 多列布局

```javascript
// 等宽列
sections: [{
  properties: {
    column: {
      count: 2,          // 列数
      space: 720,        // 列间距 DXA(720 = 0.5 英寸)
      equalWidth: true,
      separate: true,    // 列之间有竖线
    },
  },
  children: [/* 内容自然跨列流动 */]
}]

// 自定义宽度列(equalWidth 必须为 false)
sections: [{
  properties: {
    column: {
      equalWidth: false,
      children: [
        new Column({ width: 5400, space: 720 }),
        new Column({ width: 3240 }),
      ],
    },
  },
  children: [/* 内容 */]
}]
```

使用 `type: SectionType.NEXT_COLUMN` 的新节强制分列。

### 目录

```javascript
// 关键:标题必须仅使用 HeadingLevel - 不能使用自定义样式
new TableOfContents("目录", { hyperlink: true, headingStyleRange: "1-3" })
```

### 页眉/页脚

```javascript
sections: [{
  properties: {
    page: { margin: { top: 1440, right: 1440, bottom: 1440, left: 1440 } } // 1440 = 1 英寸
  },
  headers: {
    default: new Header({ children: [new Paragraph({ children: [new TextRun("页眉")] })] })
  },
  footers: {
    default: new Footer({ children: [new Paragraph({
      children: [new TextRun("第 "), new TextRun({ children: [PageNumber.CURRENT] })]
    })] })
  },
  children: [/* 内容 */]
}]
```

### docx-js 关键规则

- **明确设置页面尺寸** - docx-js 默认使用 A4;美国文档使用美国 Letter(12240 x 15840 DXA)
- **横向:传入纵向尺寸** - docx-js 在内部交换宽度/高度;传入短边作为 `width`,长边作为 `height`,并设置 `orientation: PageOrientation.LANDSCAPE`
- **永远不要使用 `\n`** - 使用单独的 Paragraph 元素
- **永远不要使用 Unicode 项目符号** - 使用带编号配置的 `LevelFormat.BULLET`
- **PageBreak 必须在 Paragraph 中** - 独立使用会创建无效 XML
- **ImageRun 需要 `type`** - 始终指定 png/jpg 等
- **始终使用 DXA 设置表格 `width`** - 永远不要使用 `WidthType.PERCENTAGE`(在 Google Docs 中会出错)
- **表格需要双重宽度** - `columnWidths` 数组和单元格 `width`,两者必须匹配
- **表格宽度 = columnWidths 之和** - 对于 DXA,确保它们精确相加
- **始终添加单元格边距** - 使用 `margins: { top: 80, bottom: 80, left: 120, right: 120 }` 以获得可读的内边距
- **使用 `ShadingType.CLEAR`** - 表格底纹永远不要使用 SOLID
- **永远不要将表格用作分隔线/规则** - 单元格有最小高度,会渲染为空框(包括在页眉/页脚中);在 Paragraph 上使用 `border: { bottom: { style: BorderStyle.SINGLE, size: 6, color: "2E75B6", space: 1 } }` 代替。对于两列页脚,使用制表位(参见制表位部分),而不是表格
- **TOC 仅需要 HeadingLevel** - 标题段落上不能使用自定义样式
- **覆盖内置样式** - 使用精确 ID:"Heading1"、"Heading2" 等
- **包含 `outlineLevel`** - TOC 必需(H1 为 0,H2 为 1,等等)

---

## 编辑现有文档

**按顺序执行所有 3 个步骤。**

### 步骤 1:解包
```bash
python scripts/office/unpack.py document.docx unpacked/
```
提取 XML,美化打印,合并相邻的 run,并将智能引号转换为 XML 实体(`&#x201C;` 等),以便它们在编辑后保留。使用 `--merge-runs false` 跳过 run 合并。

### 步骤 2:编辑 XML

编辑 `unpacked/word/` 中的文件。参见下方 XML 参考了解模式。

**使用 "Claude" 作为作者** 进行修订跟踪和批注,除非用户明确要求使用不同的名称。

**直接使用 Edit 工具进行字符串替换。不要编写 Python 脚本。** 脚本引入不必要的复杂性。Edit 工具准确显示正在替换的内容。

**关键:对新内容使用智能引号。** 添加带撇号或引号的文本时,使用 XML 实体生成智能引号:
```xml
<!-- 使用这些实体获得专业排版 -->
<w:t>Here&#x2019;s a quote: &#x201C;Hello&#x201D;</w:t>
```
| 实体       | 字符             |
| ---------- | ---------------- |
| `&#x2018;` | '(左单引号)      |
| `&#x2019;` | '(右单引号/撇号) |
| `&#x201C;` | "(左双引号)      |
| `&#x201D;` | "(右双引号)      |

**添加批注:** 使用 `comment.py` 处理跨多个 XML 文件的样板文本(文本必须是预转义的 XML):
```bash
python scripts/comment.py unpacked/ 0 "带 &amp; 和 &#x2019; 的批注文本"
python scripts/comment.py unpacked/ 1 "回复文本" --parent 0  # 回复批注 0
python scripts/comment.py unpacked/ 0 "文本" --author "自定义作者"  # 自定义作者名称
```
然后将标记添加到 document.xml(参见 XML 参考中的批注部分)。

### 步骤 3:打包
```bash
python scripts/office/pack.py unpacked/ output.docx --original document.docx
```
带自动修复验证,压缩 XML 并创建 DOCX。使用 `--validate false` 跳过。

**自动修复将修复:**
- `durableId` >= 0x7FFFFFFF(重新生成有效 ID)
- `<w:t>` 上缺少 `xml:space="preserve"`(带空白时)

**自动修复不会修复:**
- 格式错误的 XML、无效的元素嵌套、缺失的关系、架构违规

### 常见陷阱

- **替换整个 `<w:r>` 元素**:添加修订跟踪时,用 `<w:del>...<w:ins>...` 作为兄弟替换整个 `<w:r>...</w:r>` 块。不要在 run 内注入修订跟踪标签。
- **保留 `<w:rPr>` 格式**:将原始 run 的 `<w:rPr>` 块复制到你的修订跟踪 run 中,以保持粗体、字体大小等。

---

## XML 参考

### 架构合规性

- **`<w:pPr>` 中的元素顺序**:`<w:pStyle>`、`<w:numPr>`、`<w:spacing>`、`<w:ind>`、`<w:jc>`、`<w:rPr>` 最后
- **空白**:在有前导/尾随空白的 `<w:t>` 上添加 `xml:space="preserve"`
- **RSID**:必须是 8 位十六进制(例如,`00AB1234`)

### 修订跟踪

**插入:**
```xml
<w:ins w:id="1" w:author="Claude" w:date="2025-01-01T00:00:00Z">
  <w:r><w:t>插入的文本</w:t></w:r>
</w:ins>
```

**删除:**
```xml
<w:del w:id="2" w:author="Claude" w:date="2025-01-01T00:00:00Z">
  <w:r><w:delText>删除的文本</w:delText></w:r>
</w:del>
```

**在 `<w:del>` 内**:使用 `<w:delText>` 代替 `<w:t>`,使用 `<w:delInstrText>` 代替 `<w:instrText>`。

**最小化编辑** - 只标记变化的内容:
```xml
<!-- 将 "30 天" 改为 "60 天" -->
<w:r><w:t>期限是 </w:t></w:r>
<w:del w:id="1" w:author="Claude" w:date="...">
  <w:r><w:delText>30</w:delText></w:r>
</w:del>
<w:ins w:id="2" w:author="Claude" w:date="...">
  <w:r><w:t>60</w:t></w:r>
</w:ins>
<w:r><w:t> 天。</w:t></w:r>
```

**删除整个段落/列表项** - 当从段落中删除所有内容时,也将段落标记标记为已删除,以便它与下一个段落合并。在 `<w:pPr><w:rPr>` 内添加 `<w:del/>`:
```xml
<w:p>
  <w:pPr>
    <w:numPr>...</w:numPr>  <!-- 列表编号(如果存在) -->
    <w:rPr>
      <w:del w:id="1" w:author="Claude" w:date="2025-01-01T00:00:00Z"/>
    </w:rPr>
  </w:pPr>
  <w:del w:id="2" w:author="Claude" w:date="2025-01-01T00:00:00Z">
    <w:r><w:delText>正在删除的整个段落内容...</w:delText></w:r>
  </w:del>
</w:p>
```
如果没有 `<w:pPr><w:rPr>` 中的 `<w:del/>`,接受更改会留下空的段落/列表项。

**拒绝其他作者的插入** - 将删除嵌套在他们的插入内:
```xml
<w:ins w:author="Jane" w:id="5">
  <w:del w:author="Claude" w:id="10">
    <w:r><w:delText>他们插入的文本</w:delText></w:r>
  </w:del>
</w:ins>
```

**恢复其他作者的删除** - 在之后添加插入(不要修改他们的删除):
```xml
<w:del w:author="Jane" w:id="5">
  <w:r><w:delText>删除的文本</w:delText></w:r>
</w:del>
<w:ins w:author="Claude" w:id="10">
  <w:r><w:t>删除的文本</w:t></w:r>
</w:ins>
```

### 批注

运行 `comment.py`(参见步骤 2)后,将标记添加到 document.xml。对于回复,使用 `--parent` 标志并将标记嵌套在父标记内。

**关键:`<w:commentRangeStart>` 和 `<w:commentRangeEnd>` 是 `<w:r>` 的兄弟,永远不在 `<w:r>` 内。**

```xml
<!-- 批注标记是 w:p 的直接子元素,永远不在 w:r 内 -->
<w:commentRangeStart w:id="0"/>
<w:del w:id="1" w:author="Claude" w:date="2025-01-01T00:00:00Z">
  <w:r><w:delText>已删除</w:delText></w:r>
</w:del>
<w:r><w:t> 更多文本</w:t></w:r>
<w:commentRangeEnd w:id="0"/>
<w:r><w:rPr><w:rStyle w:val="CommentReference"/></w:rPr><w:commentReference w:id="0"/></w:r>

<!-- 批注 0,回复 1 嵌套在内 -->
<w:commentRangeStart w:id="0"/>
  <w:commentRangeStart w:id="1"/>
  <w:r><w:t>文本</w:t></w:r>
  <w:commentRangeEnd w:id="1"/>
<w:commentRangeEnd w:id="0"/>
<w:r><w:rPr><w:rStyle w:val="CommentReference"/></w:rPr><w:commentReference w:id="0"/></w:r>
<w:r><w:rPr><w:rStyle w:val="CommentReference"/></w:rPr><w:commentReference w:id="1"/></w:r>
```

### 图片

1. 将图片文件添加到 `word/media/`
2. 将关系添加到 `word/_rels/document.xml.rels`:
```xml
<Relationship Id="rId5" Type=".../image" Target="media/image1.png"/>
```
3. 将内容类型添加到 `[Content_Types].xml`:
```xml
<Default Extension="png" ContentType="image/png"/>
```
4. 在 document.xml 中引用:
```xml
<w:drawing>
  <wp:inline>
    <wp:extent cx="914400" cy="914400"/>  <!-- EMU:914400 = 1 英寸 -->
    <a:graphic>
      <a:graphicData uri=".../picture">
        <pic:pic>
          <pic:blipFill><a:blip r:embed="rId5"/></pic:blipFill>
        </pic:pic>
      </a:graphicData>
    </a:graphic>
  </wp:inline>
</w:drawing>
```

---

## 依赖项

- **pandoc**:文本提取
- **docx**:`npm install -g docx`(新文档)
- **LibreOffice**:PDF 转换(通过 `scripts/office/soffice.py` 为沙箱环境自动配置)
- **Poppler**:`pdftoppm` 用于图片
