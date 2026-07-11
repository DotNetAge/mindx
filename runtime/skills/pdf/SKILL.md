---
name: pdf
description: 当用户想要对 PDF 文件进行任何操作时使用此技能。包括从 PDF 中读取或提取文本/表格、将多个 PDF 合并为一个、拆分 PDF、旋转页面、添加水印、创建新 PDF、填写 PDF 表单、加密/解密 PDF、提取图片,以及对扫描的 PDF 进行 OCR 识别使其可搜索。如果用户提到 .pdf 文件或要求生成 PDF,请使用此技能。
license: Proprietary. LICENSE.txt has complete terms
metadata:
  name_zh: PDF 处理
  name_zh-tw: PDF 處理
  description_zh: PDF 文件的读取、合并、拆分、旋转、水印、创建、加密和 OCR 识别
  description_zh-tw: PDF 檔案的讀取、合併、拆分、旋轉、浮水印、建立、加密和 OCR 辨識
---

# PDF 处理指南

## 概述

本指南介绍如何使用 Python 库和命令行工具处理 PDF 文件，包括基本操作。高级功能、JavaScript 库和详细示例请查看 REFERENCE.md。填写 PDF 表单请阅读 FORMS.md 并按说明操作。

## 快速开始

```python
from pypdf import PdfReader, PdfWriter

# 读取 PDF
reader = PdfReader("document.pdf")
print(f"页数:{len(reader.pages)}")

# 提取文本
text = ""
for page in reader.pages:
    text += page.extract_text()
```

## Python 库

### pypdf - 基本操作

#### 合并 PDF
```python
from pypdf import PdfWriter, PdfReader

writer = PdfWriter()
for pdf_file in ["doc1.pdf", "doc2.pdf", "doc3.pdf"]:
    reader = PdfReader(pdf_file)
    for page in reader.pages:
        writer.add_page(page)

with open("merged.pdf", "wb") as output:
    writer.write(output)
```

#### 拆分 PDF
```python
reader = PdfReader("input.pdf")
for i, page in enumerate(reader.pages):
    writer = PdfWriter()
    writer.add_page(page)
    with open(f"page_{i+1}.pdf", "wb") as output:
        writer.write(output)
```

#### 提取元数据
```python
reader = PdfReader("document.pdf")
meta = reader.metadata
print(f"标题:{meta.title}")
print(f"作者:{meta.author}")
print(f"主题:{meta.subject}")
print(f"创建者:{meta.creator}")
```

#### 旋转页面
```python
reader = PdfReader("input.pdf")
writer = PdfWriter()

page = reader.pages[0]
page.rotate(90)  # 顺时针旋转 90 度
writer.add_page(page)

with open("rotated.pdf", "wb") as output:
    writer.write(output)
```

### pdfplumber - 文本和表格提取

#### 提取带布局的文本
```python
import pdfplumber

with pdfplumber.open("document.pdf") as pdf:
    for page in pdf.pages:
        text = page.extract_text()
        print(text)
```

#### 提取表格
```python
with pdfplumber.open("document.pdf") as pdf:
    for i, page in enumerate(pdf.pages):
        tables = page.extract_tables()
        for j, table in enumerate(tables):
            print(f"第 {i+1} 页的表格 {j+1}:")
            for row in table:
                print(row)
```

#### 高级表格提取
```python
import pandas as pd

with pdfplumber.open("document.pdf") as pdf:
    all_tables = []
    for page in pdf.pages:
        tables = page.extract_tables()
        for table in tables:
            if table:  # 检查表格是否为空
                df = pd.DataFrame(table[1:], columns=table[0])
                all_tables.append(df)

# 合并所有表格
if all_tables:
    combined_df = pd.concat(all_tables, ignore_index=True)
    combined_df.to_excel("extracted_tables.xlsx", index=False)
```

### reportlab - 创建 PDF

#### 基本 PDF 创建
```python
from reportlab.lib.pagesizes import letter
from reportlab.pdfgen import canvas

c = canvas.Canvas("hello.pdf", pagesize=letter)
width, height = letter

# 添加文本
c.drawString(100, height - 100, "Hello World!")
c.drawString(100, height - 120, "这是使用 reportlab 创建的 PDF")

# 添加线条
c.line(100, height - 140, 400, height - 140)

# 保存
c.save()
```

#### 创建多页 PDF
```python
from reportlab.lib.pagesizes import letter
from reportlab.platypus import SimpleDocTemplate, Paragraph, Spacer, PageBreak
from reportlab.lib.styles import getSampleStyleSheet

doc = SimpleDocTemplate("report.pdf", pagesize=letter)
styles = getSampleStyleSheet()
story = []

# 添加内容
title = Paragraph("报告标题", styles['Title'])
story.append(title)
story.append(Spacer(1, 12))

body = Paragraph("这是报告的正文。" * 20, styles['Normal'])
story.append(body)
story.append(PageBreak())

# 第 2 页
story.append(Paragraph("第 2 页", styles['Heading1']))
story.append(Paragraph("第 2 页的内容", styles['Normal']))

# 构建 PDF
doc.build(story)
```

#### 下标和上标

**重要**：ReportLab PDF 中切勿使用 Unicode 下标/上标字符（₀₁₂₃₄₅₆₇₈₉, ⁰¹²³⁴⁵⁶⁷⁸⁹）。内置字体不含这些字形，渲染后会变成实心黑色方块。

请改用 ReportLab 的 XML 标记标签，在 Paragraph 对象中实现：
```python
from reportlab.platypus import Paragraph
from reportlab.lib.styles import getSampleStyleSheet

styles = getSampleStyleSheet()

# 下标:使用 <sub> 标签
chemical = Paragraph("H<sub>2</sub>O", styles['Normal'])

# 上标:使用 <super> 标签
squared = Paragraph("x<super>2</super> + y<super>2</super>", styles['Normal'])
```

对于画布直接绘制的文本（非 Paragraph 对象），请手动调整字体大小和位置来实现下标/上标效果，不要使用 Unicode 字符。

## 命令行工具

### pdftotext (poppler-utils)
```bash
# 提取文本
pdftotext input.pdf output.txt

# 保留原始布局提取文本
pdftotext -layout input.pdf output.txt

# 提取特定页面
pdftotext -f 1 -l 5 input.pdf output.txt  # 第 1-5 页
```

### qpdf
```bash
# 合并 PDF
qpdf --empty --pages file1.pdf file2.pdf -- merged.pdf

# 拆分页面
qpdf input.pdf --pages . 1-5 -- pages1-5.pdf
qpdf input.pdf --pages . 6-10 -- pages6-10.pdf

# 旋转页面
qpdf input.pdf output.pdf --rotate=+90:1  # 将第 1 页旋转 90 度

# 移除密码
qpdf --password=mypassword --decrypt encrypted.pdf decrypted.pdf
```

### pdftk（如已安装）
```bash
# 合并
pdftk file1.pdf file2.pdf cat output merged.pdf

# 拆分
pdftk input.pdf burst

# 旋转
pdftk input.pdf rotate 1east output rotated.pdf
```

## 常见任务

### 从扫描的 PDF 提取文本
```python
# 需要:pip install pytesseract pdf2image
import pytesseract
from pdf2image import convert_from_path

# 将 PDF 转换为图像
images = convert_from_path('scanned.pdf')

# 对每页进行 OCR
text = ""
for i, image in enumerate(images):
    text += f"第 {i+1} 页:\n"
    text += pytesseract.image_to_string(image)
    text += "\n\n"

print(text)
```

### 添加水印
```python
from pypdf import PdfReader, PdfWriter

# 创建水印(或加载现有水印)
watermark = PdfReader("watermark.pdf").pages[0]

# 应用到所有页面
reader = PdfReader("document.pdf")
writer = PdfWriter()

for page in reader.pages:
    page.merge_page(watermark)
    writer.add_page(page)

with open("watermarked.pdf", "wb") as output:
    writer.write(output)
```

### 提取图片
```bash
# 使用 pdfimages (poppler-utils)
pdfimages -j input.pdf output_prefix

# 所有图片将导出为 output_prefix-000.jpg、output_prefix-001.jpg 等
```

### 密码保护
```python
from pypdf import PdfReader, PdfWriter

reader = PdfReader("input.pdf")
writer = PdfWriter()

for page in reader.pages:
    writer.add_page(page)

# 添加密码
writer.encrypt("userpassword", "ownerpassword")

with open("encrypted.pdf", "wb") as output:
    writer.write(output)
```

## 快速参考

| 任务 | 最佳工具 | 命令/代码 |
|------|----------|-----------|
| 合并 PDF | pypdf | `writer.add_page(page)` |
| 拆分 PDF | pypdf | 每页一个文件 |
| 提取文本 | pdfplumber | `page.extract_text()` |
| 提取表格 | pdfplumber | `page.extract_tables()` |
| 创建 PDF | reportlab | Canvas 或 Platypus |
| 命令行合并 | qpdf | `qpdf --empty --pages ...` |
| OCR 扫描 PDF | pytesseract | 先转换为图像 |
| 填写 PDF 表单 | pdf-lib 或 pypdf(参见 FORMS.md) | 参见 FORMS.md |

## 后续步骤

- 高级 pypdfium2 用法，参见 REFERENCE.md
- JavaScript 库（pdf-lib），参见 REFERENCE.md
- 填写 PDF 表单，按 FORMS.md 中的说明操作
- 故障排除指南，参见 REFERENCE.md
