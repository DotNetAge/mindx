# PDF Report Generator

> 从 Markdown 文件生成带封面的 PDF 报告。

## 激活条件

当用户需要生成 PDF 报告时调用本 Skill。触发词：生成 PDF / 导出报告 / 制作 PDF / print to PDF / pdf report。

---

## 工作流程

### Step 1: 收集输入

从用户获取以下信息：
- 源 Markdown 文件路径
- 输出 PDF 文件路径（默认同目录同名 .pdf）
- 是否需要封面（默认是）

### Step 2: 生成封面

如果用户需要封面，使用内置模板生成包含标题、日期、作者的封面页。

### Step 3: 转换正文

将 Markdown 转换为 PDF 正文：
- 调用 `scripts/convert.py` 执行转换
- 保留标题层级（H1-H4）
- 表格自动适配页宽

### Step 4: 合并输出

将封面和正文合并为最终 PDF，保存到用户指定路径。

---

## 使用示例

```bash
python scripts/convert.py report.md -o report.pdf --cover
```

## 依赖

| 依赖 | 安装 |
|:---|:---|
| weasyprint | `pip install weasyprint` |
| markdown | `pip install markdown` |

## 能力边界

- 支持：.md → .pdf
- 不支持：.docx / .html / 图片直接转 PDF
- 封面模板为固定样式，不支持自定义 CSS

---

## 脚本清单

| 脚本 | 用途 |
|:---|:---|
| `scripts/convert.py` | Markdown → PDF 转换主入口 |
| `scripts/cover.py` | 封面生成模块 |

## 异常处理

转换失败时，输出错误信息并建议检查依赖安装状态。
