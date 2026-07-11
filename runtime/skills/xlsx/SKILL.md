---
name: xlsx
description: "任何时候电子表格文件是主要输入或输出时都使用此技能。这意味着以下任何任务:打开、读取、编辑或修复现有的 .xlsx、.xlsm、.csv 或 .tsv 文件(例如添加列、计算公式、格式化、图表、清洗混乱数据);从头或从其他数据源创建新的电子表格;或在表格文件格式之间转换。特别是当用户按名称或路径引用电子表格文件时触发 — 即使是随意的(如\"我下载目录里的 xlsx\") — 并且想对它进行处理或从中生成内容。也适用于将混乱的表格数据文件(格式错误的行、错位的标题、垃圾数据)清洗或重组为规范的电子表格。交付物必须是电子表格文件。当主要交付物是 Word 文档、HTML 报告、独立 Python 脚本、数据库流水线或 Google Sheets API 集成时,即使涉及表格数据,也不要触发。"
license: Proprietary. LICENSE.txt has complete terms
metadata:
  name_zh: 电子表格处理
  name_zh-tw: 電子表格處理
  description_zh: 打开、读取、编辑或创建 .xlsx、.xlsm、.csv 等电子表格文件,支持公式、格式化和图表
  description_zh-tw: 開啟、讀取、編輯或建立 .xlsx、.xlsm、.csv 等電子表格檔案,支援公式、格式化和圖表
---

# 输出要求

## 所有 Excel 文件

### 专业字体
- 除非用户另有要求，所有交付物使用统一的专业字体（如 Arial、Times New Roman）

### 零公式错误
- 交付 Excel 模型时，必须确保没有公式错误（#REF!、#DIV/0!、#VALUE!、#N/A、#NAME?）

### 保留现有模板(更新模板时)
- 研究并完全匹配现有的格式、风格和惯例
- 永远不要将标准化格式强加于已有模式的文件
- 现有模板惯例始终优先于这些指南

## 财务模型

### 颜色编码标准
除非用户或现有模板另有要求

#### 行业标准颜色约定
- **蓝色文本（RGB: 0,0,255）**：硬编码的输入值，以及用户会随场景调整的数字
- **黑色文本（RGB: 0,0,0）**：所有公式和计算结果
- **绿色文本（RGB: 0,128,0）**：从同一工作簿其他工作表引用的链接
- **红色文本（RGB: 255,0,0）**：指向其他文件的外部链接
- **黄色背景（RGB: 255,255,0）**：需要关注的关键假设或待更新的单元格

### 数字格式标准

#### 必需格式规则
- **年份**：格式化为文本字符串（如 "2024" 而非 "2,024"）
- **货币**：使用 $#,##0 格式；标题中始终注明单位（如 "收入($百万)"）
- **零值**：所有零显示为 "-"，包括百分比（如 "$#,##0;($#,##0);-"）
- **百分比**：默认 0.0% 格式（一位小数）
- **倍数**：估值倍数（EV/EBITDA、P/E）格式化为 0.0x
- **负数**：使用括号 (123) 而非负号 -123

### 公式构建规则

#### 假设放置
- 所有假设（增长率、利润率、倍数等）放在单独的假设单元格中
- 公式中使用单元格引用，不要硬编码值
- 示例：使用 =B5*(1+$B$6) 而非 =B5*1.05

#### 公式错误预防
- 验证所有单元格引用正确
- 检查范围中的偏移一位错误
- 确保所有预测期间的公式一致
- 用边界情况测试（零值、负数）
- 确认没有意外的循环引用

#### 硬编码值的文档要求
- 在单元格中或旁边添加注释（如果在表格末尾）。格式："来源:[系统/文档],[日期],[具体引用],[URL（如适用）]"
- 示例：
  - "来源：公司 10-K，FY2024，第 45 页，收入注释，[SEC EDGAR URL]"
  - "来源：公司 10-Q，Q2 2025，附件 99.1，[SEC EDGAR URL]"
  - "来源：彭博终端，2025/8/15，AAPL US Equity"
  - "来源：FactSet，2025/8/20，一致预期数据"

# XLSX 创建、编辑和分析

## 概述

用户可能要求你创建、编辑或分析 .xlsx 文件。根据任务不同，选用不同的工具和工作流。

## 重要要求

**LibreOffice 用于公式重新计算**:你可以假设 LibreOffice 已安装,用于使用 `scripts/recalc.py` 脚本重新计算公式值。该脚本在首次运行时自动配置 LibreOffice,包括在 Unix 套接字受限的沙箱环境中(由 `scripts/office/soffice.py` 处理)

## 读取和分析数据

### 使用 pandas 进行数据分析
数据分析、可视化和基本操作使用 **pandas**，它提供强大的数据操作能力：

```python
import pandas as pd

# 读取 Excel
df = pd.read_excel('file.xlsx')  # 默认:第一个工作表
all_sheets = pd.read_excel('file.xlsx', sheet_name=None)  # 所有工作表作为字典

# 分析
df.head()      # 预览数据
df.info()      # 列信息
df.describe()  # 统计信息

# 写入 Excel
df.to_excel('output.xlsx', index=False)
```

## Excel 文件工作流

## 关键:使用公式,而非硬编码值

**始终使用 Excel 公式,而非在 Python 中计算值后硬编码。** 这确保电子表格保持动态和可更新。

### ❌ 错误 - 硬编码计算值
```python
# 不好:在 Python 中计算并硬编码结果
total = df['Sales'].sum()
sheet['B10'] = total  # 硬编码 5000

# 不好:在 Python 中计算增长率
growth = (df.iloc[-1]['Revenue'] - df.iloc[0]['Revenue']) / df.iloc[0]['Revenue']
sheet['C5'] = growth  # 硬编码 0.15

# 不好:在 Python 中计算平均值
avg = sum(values) / len(values)
sheet['D20'] = avg  # 硬编码 42.5
```

### ✅ 正确 - 使用 Excel 公式
```python
# 好:让 Excel 计算总和
sheet['B10'] = '=SUM(B2:B9)'

# 好:增长率作为 Excel 公式
sheet['C5'] = '=(C4-C2)/C2'

# 好:使用 Excel 函数计算平均值
sheet['D20'] = '=AVERAGE(D2:D19)'
```

所有计算都适用这一原则——总计、百分比、比率、差值等。源数据变更时，电子表格应能自动重新计算。

## 常见工作流
1. **选择工具**：数据分析用 pandas，公式/格式化用 openpyxl
2. **创建/加载**：创建新工作簿或加载现有文件
3. **修改**：添加/编辑数据、公式和格式
4. **保存**：写入文件
5. **重新计算公式（使用公式时必需）**：使用 scripts/recalc.py 脚本
   ```bash
   python scripts/recalc.py output.xlsx
   ```
6. **验证并修复错误**：
   - 脚本返回包含错误详情的 JSON
   - 如果 `status` 为 `errors_found`，检查 `error_summary` 了解具体错误类型和位置
   - 修复已识别的错误并重新计算
   - 常见需修复的错误：
     - `#REF!`：无效的单元格引用
     - `#DIV/0!`：除以零
     - `#VALUE!`：公式中数据类型错误
     - `#NAME?`：无法识别的公式名称

### 创建新的 Excel 文件

```python
# 使用 openpyxl 进行公式和格式化
from openpyxl import Workbook
from openpyxl.styles import Font, PatternFill, Alignment

wb = Workbook()
sheet = wb.active

# 添加数据
sheet['A1'] = 'Hello'
sheet['B1'] = 'World'
sheet.append(['Row', 'of', 'data'])

# 添加公式
sheet['B2'] = '=SUM(A1:A10)'

# 格式化
sheet['A1'].font = Font(bold=True, color='FF0000')
sheet['A1'].fill = PatternFill('solid', start_color='FFFF00')
sheet['A1'].alignment = Alignment(horizontal='center')

# 列宽
sheet.column_dimensions['A'].width = 20

wb.save('output.xlsx')
```

### 编辑现有的 Excel 文件

```python
# 使用 openpyxl 保留公式和格式
from openpyxl import load_workbook

# 加载现有文件
wb = load_workbook('existing.xlsx')
sheet = wb.active  # 或 wb['SheetName'] 指定工作表

# 处理多个工作表
for sheet_name in wb.sheetnames:
    sheet = wb[sheet_name]
    print(f"工作表: {sheet_name}")

# 修改单元格
sheet['A1'] = '新值'
sheet.insert_rows(2)  # 在位置 2 插入行
sheet.delete_cols(3)  # 删除第 3 列

# 添加新工作表
new_sheet = wb.create_sheet('新工作表')
new_sheet['A1'] = '数据'

wb.save('modified.xlsx')
```

## 重新计算公式

通过 openpyxl 创建或修改的 Excel 文件，公式以字符串形式保存，但不包含计算值。使用 `scripts/recalc.py` 脚本重新计算公式：

```bash
python scripts/recalc.py <excel_file> [timeout_seconds]
```

示例：
```bash
python scripts/recalc.py output.xlsx 30
```

该脚本功能：
- 首次运行时自动设置 LibreOffice 宏
- 重新计算所有工作表中的所有公式
- 扫描所有单元格，查找 Excel 错误（#REF!、#DIV/0! 等）
- 返回包含详细错误位置和计数的 JSON
- 支持 Linux 和 macOS

## 公式验证清单

快速检查，确保公式正常工作：

### 基本验证
- [ ] **测试 2-3 个样本引用**：构建完整模型前，验证它们能获取正确的值
- [ ] **列映射**：确认 Excel 列对应正确（例如，第 64 列 = BL，不是 BK）
- [ ] **行偏移**：记住 Excel 行从 1 开始索引（DataFrame 第 5 行 = Excel 第 6 行）

### 常见陷阱
- [ ] **NaN 处理**：使用 `pd.notna()` 检查空值
- [ ] **最右侧列**：财务数据通常在第 50+ 列
- [ ] **多个匹配**：搜索所有出现项，不只是第一个
- [ ] **除以零**：公式中使用 `/` 前，检查分母（#DIV/0!）
- [ ] **错误引用**：验证所有单元格引用指向预期单元格（#REF!）
- [ ] **跨工作表引用**：使用正确格式（Sheet1!A1）链接工作表

### 公式测试策略
- [ ] **从小处开始**：广泛使用前，先在 2-3 个单元格上测试公式
- [ ] **验证依赖项**：检查公式中引用的所有单元格是否存在
- [ ] **测试边界情况**：包括零值、负值和非常大的值

### 解读 scripts/recalc.py 输出
脚本返回包含错误详情的 JSON：
```json
{
  "status": "success",           // 或 "errors_found"
  "total_errors": 0,              // 总错误数
  "total_formulas": 42,           // 文件中的公式数量
  "error_summary": {              // 仅在发现错误时存在
    "#REF!": {
      "count": 2,
      "locations": ["Sheet1!B5", "Sheet1!C10"]
    }
  }
}
```

## 最佳实践

### 库选择
- **pandas**：最适合数据分析、批量操作和简单数据导出
- **openpyxl**：最适合复杂格式化、公式和 Excel 特定功能

### 使用 openpyxl
- 单元格索引从 1 开始（row=1, column=1 指 A1 单元格）
- 使用 `data_only=True` 读取计算值：`load_workbook('file.xlsx', data_only=True)`
- **警告**：用 `data_only=True` 打开并保存时，公式会被值替换，永久丢失
- 大文件处理：读取用 `read_only=True`，写入用 `write_only=True`
- 公式会保留但不求值——使用 scripts/recalc.py 更新值

### 使用 pandas
- 指定数据类型，避免推断问题：`pd.read_excel('file.xlsx', dtype={'id': str})`
- 大文件读取特定列：`pd.read_excel('file.xlsx', usecols=['A', 'C', 'E'])`
- 正确处理日期：`pd.read_excel('file.xlsx', parse_dates=['date_column'])`

## 代码风格指南
**重要**：为 Excel 操作生成 Python 代码时：
- 编写简洁的 Python 代码，不带不必要的注释
- 避免冗长的变量名和冗余操作
- 避免不必要的 print 语句

**Excel 文件本身**：
- 为复杂公式或重要假设的单元格添加注释
- 记录硬编码值的数据来源
- 为关键计算和模型部分添加说明
