# PptxGenJS 教程

## 环境配置与基本结构

```javascript
const pptxgen = require("pptxgenjs");

let pres = new pptxgen();
pres.layout = 'LAYOUT_16x9';  // 或 'LAYOUT_16x10', 'LAYOUT_4x3', 'LAYOUT_WIDE'
pres.author = 'Your Name';
pres.title = 'Presentation Title';

let slide = pres.addSlide();
slide.addText("Hello World!", { x: 0.5, y: 0.5, fontSize: 36, color: "363636" });

pres.writeFile({ fileName: "Presentation.pptx" });
```

## 布局尺寸

幻灯片尺寸（坐标单位为英寸）：
- `LAYOUT_16x9`：10" × 5.625"（默认）
- `LAYOUT_16x10`：10" × 6.25"
- `LAYOUT_4x3`：10" × 7.5"
- `LAYOUT_WIDE`：13.3" × 7.5"

---

## 文本与格式

```javascript
// 基本文本
slide.addText("Simple Text", {
  x: 1, y: 1, w: 8, h: 2, fontSize: 24, fontFace: "Arial",
  color: "363636", bold: true, align: "center", valign: "middle"
});

// 字符间距（使用 charSpacing，letterSpacing 会被静默忽略）
slide.addText("SPACED TEXT", { x: 1, y: 1, w: 8, h: 1, charSpacing: 6 });

// 富文本数组
slide.addText([
  { text: "Bold ", options: { bold: true } },
  { text: "Italic ", options: { italic: true } }
], { x: 1, y: 3, w: 8, h: 1 });

// 多行文本（需要 breakLine: true）
slide.addText([
  { text: "Line 1", options: { breakLine: true } },
  { text: "Line 2", options: { breakLine: true } },
  { text: "Line 3" }  // 最后一项不需要 breakLine
], { x: 0.5, y: 0.5, w: 8, h: 2 });

// 文本框内边距
slide.addText("Title", {
  x: 0.5, y: 0.3, w: 9, h: 0.6,
  margin: 0  // 当需要将文本与形状或图标等元素对齐时使用 0
});
```

**提示**：文本框默认有内边距。当需要文本与同 x 位置的形状、线条或图标精确对齐时，设置 `margin: 0`。

---

## 列表与项目符号

```javascript
// ✅ 正确：多个项目符号
slide.addText([
  { text: "First item", options: { bullet: true, breakLine: true } },
  { text: "Second item", options: { bullet: true, breakLine: true } },
  { text: "Third item", options: { bullet: true } }
], { x: 0.5, y: 0.5, w: 8, h: 3 });

// ❌ 错误：不要使用 unicode 项目符号
slide.addText("• First item", { ... });  // 会产生双重项目符号

// 子项和编号列表
{ text: "Sub-item", options: { bullet: true, indentLevel: 1 } }
{ text: "First", options: { bullet: { type: "number" }, breakLine: true } }
```

---

## 形状

```javascript
slide.addShape(pres.shapes.RECTANGLE, {
  x: 0.5, y: 0.8, w: 1.5, h: 3.0,
  fill: { color: "FF0000" }, line: { color: "000000", width: 2 }
});

slide.addShape(pres.shapes.OVAL, { x: 4, y: 1, w: 2, h: 2, fill: { color: "0000FF" } });

slide.addShape(pres.shapes.LINE, {
  x: 1, y: 3, w: 5, h: 0, line: { color: "FF0000", width: 3, dashType: "dash" }
});

// 带透明度
slide.addShape(pres.shapes.RECTANGLE, {
  x: 1, y: 1, w: 3, h: 2,
  fill: { color: "0088CC", transparency: 50 }
});

// 圆角矩形（rectRadius 仅对 ROUNDED_RECTANGLE 有效，对 RECTANGLE 无效）
// ⚠️ 不要与矩形强调叠加层搭配使用 —— 它们无法覆盖圆角。改用 RECTANGLE。
slide.addShape(pres.shapes.ROUNDED_RECTANGLE, {
  x: 1, y: 1, w: 3, h: 2,
  fill: { color: "FFFFFF" }, rectRadius: 0.1
});

// 带阴影
slide.addShape(pres.shapes.RECTANGLE, {
  x: 1, y: 1, w: 3, h: 2,
  fill: { color: "FFFFFF" },
  shadow: { type: "outer", color: "000000", blur: 6, offset: 2, angle: 135, opacity: 0.15 }
});
```

阴影参数：

| 属性 | 类型 | 范围 | 说明 |
|----------|------|-------|-------|
| `type` | string | `"outer"`、`"inner"` | |
| `color` | string | 6 位十六进制（如 `"000000"`） | 不加 `#` 前缀，不要用 8 位十六进制 —— 详见常见陷阱 |
| `blur` | number | 0-100 pt | |
| `offset` | number | 0-200 pt | **必须为非负数** —— 负值会导致文件损坏 |
| `angle` | number | 0-359 度 | 阴影投射方向（135 = 右下方，270 = 正上方） |
| `opacity` | number | 0.0-1.0 | 用此属性控制透明度，不要在颜色字符串中编码 |

要让阴影向上投射（例如页脚横条），使用 `angle: 270` 配合正值 offset —— **不要**使用负 offset。

**注意**：不支持原生渐变填充。如需渐变效果，请使用渐变图片作为背景。

---

## 图片

### 图片来源

```javascript
// 从文件路径
slide.addImage({ path: "images/chart.png", x: 1, y: 1, w: 5, h: 3 });

// 从 URL
slide.addImage({ path: "https://example.com/image.jpg", x: 1, y: 1, w: 5, h: 3 });

// 从 base64（更快，无文件 I/O）
slide.addImage({ data: "image/png;base64,iVBORw0KGgo...", x: 1, y: 1, w: 5, h: 3 });
```

### 图片选项

```javascript
slide.addImage({
  path: "image.png",
  x: 1, y: 1, w: 5, h: 3,
  rotate: 45,              // 0-359 度
  rounding: true,          // 圆形裁剪
  transparency: 50,        // 0-100
  flipH: true,             // 水平翻转
  flipV: false,            // 垂直翻转
  altText: "Description",  // 无障碍文本
  hyperlink: { url: "https://example.com" }
});
```

### 图片尺寸模式

```javascript
// Contain - 在区域内完整显示，保持宽高比
{ sizing: { type: 'contain', w: 4, h: 3 } }

// Cover - 填满区域，保持宽高比（可能裁剪）
{ sizing: { type: 'cover', w: 4, h: 3 } }

// Crop - 裁剪指定区域
{ sizing: { type: 'crop', x: 0.5, y: 0.5, w: 2, h: 2 } }
```

### 计算尺寸（保持宽高比）

```javascript
const origWidth = 1978, origHeight = 923, maxHeight = 3.0;
const calcWidth = maxHeight * (origWidth / origHeight);
const centerX = (10 - calcWidth) / 2;

slide.addImage({ path: "image.png", x: centerX, y: 1.2, w: calcWidth, h: maxHeight });
```

### 支持的格式

- **标准格式**：PNG、JPG、GIF（动态 GIF 在 Microsoft 365 中可用）
- **SVG**：在现代版 PowerPoint / Microsoft 365 中可用

---

## 图标

使用 react-icons 生成 SVG 图标，然后光栅化为 PNG 以确保通用兼容性。

### 环境配置

```javascript
const React = require("react");
const ReactDOMServer = require("react-dom/server");
const sharp = require("sharp");
const { FaCheckCircle, FaChartLine } = require("react-icons/fa");

function renderIconSvg(IconComponent, color = "#000000", size = 256) {
  return ReactDOMServer.renderToStaticMarkup(
    React.createElement(IconComponent, { color, size: String(size) })
  );
}

async function iconToBase64Png(IconComponent, color, size = 256) {
  const svg = renderIconSvg(IconComponent, color, size);
  const pngBuffer = await sharp(Buffer.from(svg)).png().toBuffer();
  return "image/png;base64," + pngBuffer.toString("base64");
}
```

### 添加图标到幻灯片

```javascript
const iconData = await iconToBase64Png(FaCheckCircle, "#4472C4", 256);

slide.addImage({
  data: iconData,
  x: 1, y: 1, w: 0.5, h: 0.5  // 尺寸单位为英寸
});
```

**注意**：使用 256 或更大的尺寸以获得清晰的图标。size 参数控制光栅化分辨率，而非幻灯片上的显示尺寸（显示尺寸由 `w` 和 `h` 以英寸为单位设置）。

### 图标库

安装：`npm install -g react-icons react react-dom sharp`

react-icons 中常用的图标集：
- `react-icons/fa` - Font Awesome
- `react-icons/md` - Material Design
- `react-icons/hi` - Heroicons
- `react-icons/bi` - Bootstrap Icons

---

## 幻灯片背景

```javascript
// 纯色
slide.background = { color: "F1F1F1" };

// 带透明度的颜色
slide.background = { color: "FF3399", transparency: 50 };

// 从 URL 加载图片
slide.background = { path: "https://example.com/bg.jpg" };

// 从 base64 加载图片
slide.background = { data: "image/png;base64,iVBORw0KGgo..." };
```

---

## 表格

```javascript
slide.addTable([
  ["Header 1", "Header 2"],
  ["Cell 1", "Cell 2"]
], {
  x: 1, y: 1, w: 8, h: 2,
  border: { pt: 1, color: "999999" }, fill: { color: "F1F1F1" }
});

// 进阶用法：合并单元格
let tableData = [
  [{ text: "Header", options: { fill: { color: "6699CC" }, color: "FFFFFF", bold: true } }, "Cell"],
  [{ text: "Merged", options: { colspan: 2 } }]
];
slide.addTable(tableData, { x: 1, y: 3.5, w: 8, colW: [4, 4] });
```

---

## 图表

```javascript
// 柱状图
slide.addChart(pres.charts.BAR, [{
  name: "Sales", labels: ["Q1", "Q2", "Q3", "Q4"], values: [4500, 5500, 6200, 7100]
}], {
  x: 0.5, y: 0.6, w: 6, h: 3, barDir: 'col',
  showTitle: true, title: 'Quarterly Sales'
});

// 折线图
slide.addChart(pres.charts.LINE, [{
  name: "Temp", labels: ["Jan", "Feb", "Mar"], values: [32, 35, 42]
}], { x: 0.5, y: 4, w: 6, h: 3, lineSize: 3, lineSmooth: true });

// 饼图
slide.addChart(pres.charts.PIE, [{
  name: "Share", labels: ["A", "B", "Other"], values: [35, 45, 20]
}], { x: 7, y: 1, w: 5, h: 4, showPercent: true });
```

### 更美观的图表

默认图表样式比较过时。应用以下选项可以获得现代、简洁的外观：

```javascript
slide.addChart(pres.charts.BAR, chartData, {
  x: 0.5, y: 1, w: 9, h: 4, barDir: "col",

  // 自定义颜色（与演示文稿配色方案一致）
  chartColors: ["0D9488", "14B8A6", "5EEAD4"],

  // 干净的背景
  chartArea: { fill: { color: "FFFFFF" }, roundedCorners: true },

  // 柔和的轴标签
  catAxisLabelColor: "64748B",
  valAxisLabelColor: "64748B",

  // 淡色网格线（仅值轴）
  valGridLine: { color: "E2E8F0", size: 0.5 },
  catGridLine: { style: "none" },

  // 在柱状图上显示数据标签
  showValue: true,
  dataLabelPosition: "outEnd",
  dataLabelColor: "1E293B",

  // 单系列时隐藏图例
  showLegend: false,
});
```

**关键样式选项**：
- `chartColors: [...]` - 系列/扇区的十六进制颜色
- `chartArea: { fill, border, roundedCorners }` - 图表背景
- `catGridLine/valGridLine: { color, style, size }` - 网格线（`style: "none"` 隐藏）
- `lineSmooth: true` - 曲线（折线图）
- `legendPos: "r"` - 图例位置："b"、"t"、"l"、"r"、"tr"

---

## 幻灯片母版

```javascript
pres.defineSlideMaster({
  title: 'TITLE_SLIDE', background: { color: '283A5E' },
  objects: [{
    placeholder: { options: { name: 'title', type: 'title', x: 1, y: 2, w: 8, h: 2 } }
  }]
});

let titleSlide = pres.addSlide({ masterName: "TITLE_SLIDE" });
titleSlide.addText("My Title", { placeholder: "title" });
```

---

## 常见陷阱

⚠️ 以下问题会导致文件损坏、视觉错误或输出异常，请务必避免。

1. **十六进制颜色不要加 "#"** —— 会导致文件损坏
   ```javascript
   color: "FF0000"      // ✅ 正确
   color: "#FF0000"     // ❌ 错误
   ```

2. **不要在十六进制颜色字符串中编码透明度** —— 8 位颜色（如 `"00000020"`）会导致文件损坏。请使用 `opacity` 属性。
   ```javascript
   shadow: { type: "outer", blur: 6, offset: 2, color: "00000020" }          // ❌ 文件损坏
   shadow: { type: "outer", blur: 6, offset: 2, color: "000000", opacity: 0.12 }  // ✅ 正确
   ```

3. **使用 `bullet: true`** —— 不要使用 "•" 等 unicode 符号（会产生双重项目符号）

4. **数组项之间使用 `breakLine: true`** 或文本连续排列时

5. **项目符号避免使用 `lineSpacing`** —— 会导致间距过大；改用 `paraSpaceAfter`

6. **每个演示文稿需要全新的实例** —— 不要复用 `pptxgen()` 对象

7. **不要在多次调用之间复用选项对象** —— PptxGenJS 会就地修改对象（例如将阴影值转换为 EMU）。在多次调用之间共享同一个对象会导致第二个形状损坏。
   ```javascript
   const shadow = { type: "outer", blur: 6, offset: 2, color: "000000", opacity: 0.15 };
   slide.addShape(pres.shapes.RECTANGLE, { shadow, ... });  // ❌ 第二次调用拿到的是已转换的值
   slide.addShape(pres.shapes.RECTANGLE, { shadow, ... });

   const makeShadow = () => ({ type: "outer", blur: 6, offset: 2, color: "000000", opacity: 0.15 });
   slide.addShape(pres.shapes.RECTANGLE, { shadow: makeShadow(), ... });  // ✅ 每次都创建新对象
   slide.addShape(pres.shapes.RECTANGLE, { shadow: makeShadow(), ... });
   ```

8. **圆角矩形不要搭配强调边框使用** —— 矩形叠加条无法覆盖圆角。改用 `RECTANGLE`。
   ```javascript
   // ❌ 错误：强调条无法覆盖圆角
   slide.addShape(pres.shapes.ROUNDED_RECTANGLE, { x: 1, y: 1, w: 3, h: 1.5, fill: { color: "FFFFFF" } });
   slide.addShape(pres.shapes.RECTANGLE, { x: 1, y: 1, w: 0.08, h: 1.5, fill: { color: "0891B2" } });

   // ✅ 正确：使用 RECTANGLE 确保对齐干净
   slide.addShape(pres.shapes.RECTANGLE, { x: 1, y: 1, w: 3, h: 1.5, fill: { color: "FFFFFF" } });
   slide.addShape(pres.shapes.RECTANGLE, { x: 1, y: 1, w: 0.08, h: 1.5, fill: { color: "0891B2" } });
   ```

---

## 快速参考

- **形状**：RECTANGLE、OVAL、LINE、ROUNDED_RECTANGLE
- **图表**：BAR、LINE、PIE、DOUGHNUT、SCATTER、BUBBLE、RADAR
- **布局**：LAYOUT_16x9（10"×5.625"）、LAYOUT_16x10、LAYOUT_4x3、LAYOUT_WIDE
- **对齐**："left"、"center"、"right"
- **图表数据标签位置**："outEnd"、"inEnd"、"center"
