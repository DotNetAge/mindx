---
updated: 2026-10-25
scope: 初版六样板，覆盖小微到规模化的常见形态；每份为可发布的最小合法树
---

# 组织架构样板库

按老板选定的类型取用对应样板：改组织名 → 增删改节点（占位名换真名或保留）→ 老板确认后把已确认人员的可寻址账号填进 account。所有 account 初始留空（空位）。节点 name 必填；title/account 可空。

## 目录

- 微型合伙（partners）：3-8 人扁平
- 初创公司（startup）：CEO 四部门
- 标准中小企业（standard）：五部两层
- 贸易电商（trading）：供应链+平台+客服
- 连锁门店（chain）：总部+多门店
- 科技研发（tech）：CTO 五组

## 微型合伙（partners）

适用：小工作室、合伙生意，负责人直管全部职能。

```xml
<org name="合伙商号">
  <member name="负责人" title="负责人">
    <member name="商务" title="商务"/>
    <member name="交付" title="交付"/>
    <member name="财务" title="财务"/>
    <member name="行政" title="行政"/>
  </member>
</org>
```

## 初创公司（startup）

适用：10-40 人，CEO 直管四条线，各线负责人带少量专员。

```xml
<org name="初创公司">
  <member name="CEO" title="首席执行官">
    <member name="产品技术负责人" title="产品技术负责人">
      <member name="产品经理" title="产品经理"/>
      <member name="开发工程师" title="开发工程师"/>
      <member name="测试工程师" title="测试工程师"/>
    </member>
    <member name="市场负责人" title="市场负责人">
      <member name="市场专员" title="市场专员"/>
      <member name="新媒体运营" title="新媒体运营"/>
    </member>
    <member name="运营负责人" title="运营负责人">
      <member name="售后服务" title="售后服务"/>
      <member name="实施工程师" title="实施工程师"/>
    </member>
    <member name="财务行政经理" title="财务行政经理">
      <member name="会计" title="会计"/>
      <member name="行政人事" title="行政人事"/>
    </member>
  </member>
</org>
```

## 标准中小企业（standard）

适用：50-150 人制造/工程/服务混合业态，五大部门各两层。

```xml
<org name="有限公司">
  <member name="总经理" title="总经理">
    <member name="研发部经理" title="研发部经理">
      <member name="研发工程师" title="研发工程师"/>
      <member name="测试工程师" title="测试工程师"/>
    </member>
    <member name="生产部经理" title="生产部经理">
      <member name="生产主管" title="生产主管"/>
      <member name="质检员" title="质检员"/>
    </member>
    <member name="销售部经理" title="销售部经理">
      <member name="销售代表" title="销售代表"/>
      <member name="售前支持" title="售前支持"/>
    </member>
    <member name="财务部经理" title="财务部经理">
      <member name="会计" title="会计"/>
      <member name="出纳" title="出纳"/>
    </member>
    <member name="人事行政经理" title="人事行政经理">
      <member name="人事专员" title="人事专员"/>
      <member name="行政专员" title="行政专员"/>
    </member>
  </member>
</org>
```

## 贸易电商（trading）

适用：线上线下贸易、平台电商，货物流转与平台运营为核心。

```xml
<org name="贸易公司">
  <member name="总经理" title="总经理">
    <member name="采购供应链经理" title="采购供应链经理">
      <member name="采购专员" title="采购专员"/>
      <member name="供应商管理" title="供应商管理"/>
    </member>
    <member name="平台运营经理" title="平台运营经理">
      <member name="店铺运营" title="店铺运营"/>
      <member name="美工设计" title="美工设计"/>
      <member name="直播运营" title="直播运营"/>
    </member>
    <member name="客服部经理" title="客服部经理">
      <member name="客服专员" title="客服专员"/>
      <member name="售后专员" title="售后专员"/>
    </member>
    <member name="仓储物流经理" title="仓储物流经理">
      <member name="仓管员" title="仓管员"/>
      <member name="打包发货" title="打包发货"/>
    </member>
    <member name="财务经理" title="财务经理">
      <member name="会计" title="会计"/>
      <member name="出纳" title="出纳"/>
    </member>
  </member>
</org>
```

## 连锁门店（chain）

适用：餐饮/零售/服务业连锁，总部职能 + 门店复制结构。

```xml
<org name="连锁品牌">
  <member name="总经理" title="总经理">
    <member name="运营督导" title="运营督导">
      <member name="A店店长" title="A店店长">
        <member name="A店店员" title="A店店员"/>
        <member name="A店收银" title="A店收银"/>
      </member>
      <member name="B店店长" title="B店店长">
        <member name="B店店员" title="B店店员"/>
        <member name="B店收银" title="B店收银"/>
      </member>
      <member name="C店店长" title="C店店长">
        <member name="C店店员" title="C店店员"/>
        <member name="C店收银" title="C店收银"/>
      </member>
    </member>
    <member name="财务经理" title="财务经理"/>
    <member name="人事经理" title="人事经理"/>
    <member name="供应链主管" title="供应链主管">
      <member name="仓储员" title="仓储员"/>
      <member name="配送员" title="配送员"/>
    </member>
  </member>
</org>
```

## 科技研发（tech）

适用：软件/互联网研发团队，按技术栈分组长制。

```xml
<org name="科技公司">
  <member name="CTO" title="首席技术官">
    <member name="前端组长" title="前端组长">
      <member name="前端工程师" title="前端工程师"/>
      <member name="初级前端" title="前端工程师"/>
    </member>
    <member name="后端组长" title="后端组长">
      <member name="后端工程师" title="后端工程师"/>
      <member name="初级后端" title="后端工程师"/>
    </member>
    <member name="算法组长" title="算法组长">
      <member name="算法工程师" title="算法工程师"/>
    </member>
    <member name="测试组长" title="测试组长">
      <member name="测试工程师" title="测试工程师"/>
    </member>
    <member name="运维组长" title="运维组长">
      <member name="运维工程师" title="运维工程师"/>
    </member>
  </member>
</org>
```

## 改造规则

- 换组织名：`<org name="...">` 改为老板公司的名字（64 字内）
- 加部门/人：复制同结构 `<member>` 块放到对应父节点内；删去不要的节点
- 占位名换真人：把 name 改为真实姓名（title 保留职位）；没有真人的位置保留占位名
- 补账号：某位置确认了真人并拿到其可寻址账号后，`account="账号"` 填入该节点并重新发布，之后才能为该人发邀请
- 岗位用词：一律用中国老板日常叫法（仓管员、店长、会计、售后服务、实施工程师），不用舶来词与互联网黑话（客户成功、增长负责人、运营交付）——样板给八成传统企业看，黑话让人看不懂还显得外行；老板口述的原词优先于本条
