#!/usr/bin/env python3
"""仪表板脚手架生成器：读仪表板 spec（JSON），产出 .dash 单文件。

用途：标准部件为主的仪表板走脚本生成，Agent 只写 spec（数据与结构），
部件 HTML 由本脚本内建渲染，节省上下文。布局/部件高度定制时不用本脚本，
直接手写（控件写法查 references/card-cookbook.md）。

样式纪律：部件公共样式由本脚本统一提升到 board 级 <style>（class 前缀
w-*），颜色全部走 UIKit token（var(--mx-*)，亮暗主题自动跟随），卡内零
重复样式、零写死色值。gantt 例外——栅格列数是实例相关的，样式留卡内。

用法：
    python3 new_board.py spec.json -o out.dash
    cat spec.json | python3 new_board.py -o out.dash
    python3 new_board.py --help

spec 格式（JSON）：
{
  "board": {"name": "仪表板名", "gap": 12, "class": "可选 board 级类名",
            "style": "可选：追加的 board 级 CSS（自定义部件样式写这里）"},
  "cards": [
    {"widget": "stat",     "id": "s1", "class": "可选卡级类名", "span": 6, "title": "待处理",
     "value": "14", "sub": "含 1 个 P0", "tone": "danger"},
    {"widget": "progress", "id": "p1", "span": 24, "title": "迭代进度",
     "label": "总体进度 71%", "percent": 71, "tone": "ok",
     "marks": [{"left": 25, "label": "需求冻结"}]},
    {"widget": "bars",     "id": "b1", "span": 12, "title": "严重级分布",
     "rows": [{"name": "P0", "value": 1, "tone": "danger"},
              {"name": "P1", "value": 3, "tone": "warn"}]},
    {"widget": "funnel",   "id": "f1", "span": 12, "title": "招聘漏斗",
     "steps": [{"label": "简历", "value": 128}, {"label": "入职", "value": 6}]},
    {"widget": "flow",     "id": "fl1", "span": 24, "title": "缺陷流转",
     "columns": [{"name": "待确认", "items": [
        {"text": "设置页白屏", "tag": "P0", "tone": "danger", "meta": "陈晓 · 09-30"}]}]},
    {"widget": "table",    "id": "t1", "span": 24, "title": "明细",
     "columns": ["编号", "标题", "状态"],
     "rows": [["BUG-231", "导出超时", {"text": "修复中", "tone": "warn"}]]},
    {"widget": "timeline", "id": "tl1", "span": 8, "title": "里程碑",
     "items": [{"text": "需求冻结", "date": "09-12", "state": "done"},
               {"text": "全量发布", "date": "10-08", "state": "todo"}]},
    {"widget": "gantt",    "id": "g1", "span": 10, "title": "排期", "h": 300,
     "days": ["09-24", "09-25", "09-26", "09-27", "09-28", "09-29", "09-30"],
     "today": 4,
     "rows": [{"name": "李响", "bars": [
        {"from": 0, "to": 3, "label": "报表优化", "tone": "accent"}]}]},
    {"widget": "members",  "id": "m1", "span": 24, "title": "团队成员",
     "people": [{"name": "陈晓", "role": "客户端", "state": "on"}]},
    {"widget": "raw",      "id": "x1", "span": 24, "title": "自定义",
     "html": "<div>任意 HTML</div>", "html_file": "或给文件路径（如 Archify 产物）"}
  ]
}

tone 色板（token 引用，主题跟随）：ok=var(--mx-success) / warn=var(--mx-warning) /
danger=var(--mx-danger) / info=var(--mx-business) / accent=var(--mx-accent) /
neutral（current 色）。缺省 neutral。
"""
import json
import sys

TONES = {
    "ok": "var(--mx-success)",
    "warn": "var(--mx-warning)",
    "danger": "var(--mx-danger)",
    "info": "var(--mx-business)",
    "accent": "var(--mx-accent)",
    "neutral": None,
}
AVA_COLORS = ["var(--mx-accent)", "var(--mx-success)", "var(--mx-warning)", "var(--mx-business)"]
ON_ACCENT = "var(--mx-text-on-accent)"

# 部件公共样式（board 级 <style>，卡挂 w-* 类消费）。gantt 栅格列数实例相关，留卡内。
BOARD_CSS = """    /* stat 统计卡 */
    .w-stat .s{display:flex;flex-direction:column;gap:4px}
    .w-stat .s .n{font:600 28px/36px var(--mx-font-family)}
    .w-stat .s .d{font-size:12px;color:var(--mx-text-secondary)}
    /* progress 进度条 */
    .w-progress .bar{position:relative;height:10px;border-radius:5px;background:color-mix(in currentColor,12%,transparent)}
    .w-progress .bar i{display:block;height:100%;border-radius:5px}
    .w-progress .mark{position:absolute;top:-4px;width:2px;height:18px;background:color-mix(in currentColor,45%,transparent)}
    /* bars 横向条形 */
    .w-bars .row{display:flex;align-items:center;gap:10px;margin-bottom:10px;font-size:13px}
    .w-bars .row .name{width:64px;flex-shrink:0;color:var(--mx-text-secondary)}
    .w-bars .row .track{flex:1;height:10px;border-radius:5px;background:color-mix(in currentColor,10%,transparent)}
    .w-bars .row .track i{display:block;height:100%;border-radius:5px}
    .w-bars .row .v{width:28px;text-align:right;font-weight:600;flex-shrink:0}
    /* funnel 漏斗 */
    .w-funnel .f{display:flex;flex-direction:column;gap:6px;font-size:13px}
    .w-funnel .step{display:flex;align-items:center;gap:10px}
    .w-funnel .step .label{width:64px;flex-shrink:0;color:var(--mx-text-secondary);font-size:12px}
    .w-funnel .step .bar{height:22px;border-radius:6px;min-width:36px;display:flex;align-items:center;justify-content:flex-end;padding-right:8px;color:var(--mx-text-on-accent);font-size:12px;font-weight:600}
    /* flow 流转列 */
    .w-flow .cols{display:flex;gap:12px}
    .w-flow .col{flex:1;min-width:0}
    .w-flow .col h4{margin:0 0 8px;font-size:12px;font-weight:500;color:var(--mx-text-secondary)}
    .w-flow .col h4 b{font-weight:600}
    .w-flow .t{padding:8px 10px;border-radius:8px;background:color-mix(in currentColor,6%,transparent);margin-bottom:8px;font-size:13px}
    .w-flow .t .meta{display:flex;gap:6px;margin-top:6px;font-size:11px;color:var(--mx-text-tertiary);align-items:center}
    .w-flow .tag{padding:1px 8px;border-radius:999px;font-size:11px}
    /* table 表格 */
    .w-table table{width:100%;border-collapse:collapse;font-size:13px}
    .w-table th{text-align:left;font-weight:500;color:var(--mx-text-tertiary);padding:6px 10px;border-bottom:1px solid var(--mx-separator-soft);font-size:12px}
    .w-table td{padding:8px 10px;border-bottom:1px solid var(--mx-separator-soft)}
    .w-table tr:last-child td{border-bottom:none}
    .w-table .tag{padding:1px 8px;border-radius:999px;font-size:11px}
    /* timeline 时间线 */
    .w-timeline .tl{position:relative;padding-left:18px}
    .w-timeline .tl::before{content:"";position:absolute;left:5px;top:6px;bottom:6px;width:1.5px;background:color-mix(in currentColor,18%,transparent)}
    .w-timeline .m{position:relative;padding-bottom:14px;font-size:13px}
    .w-timeline .m::before{content:"";position:absolute;left:-18px;top:4px;width:9px;height:9px;border-radius:50%;background:color-mix(in currentColor,30%,transparent)}
    .w-timeline .m.done::before{background:var(--mx-success)}
    .w-timeline .m.doing::before{background:var(--mx-accent);box-shadow:0 0 0 3px color-mix(in srgb,var(--mx-accent) 18%,transparent)}
    .w-timeline .m .d{font-size:11px;color:var(--mx-text-tertiary);margin-left:6px}
    /* members 成员墙 */
    .w-members .grid{display:grid;grid-template-columns:repeat(auto-fill,minmax(150px,1fr));gap:10px}
    .w-members .m{display:flex;align-items:center;gap:10px;padding:10px 12px;border-radius:10px;background:color-mix(in currentColor,5%,transparent)}
    .w-members .ava{flex-shrink:0;width:34px;height:34px;border-radius:50%;display:flex;align-items:center;justify-content:center;font-size:13px;font-weight:600;color:var(--mx-text-on-accent)}
    .w-members .who{min-width:0}
    .w-members .who b{display:block;font-size:13px;font-weight:600}
    .w-members .who span{display:block;font-size:11px;color:var(--mx-text-secondary);white-space:nowrap;overflow:hidden;text-overflow:ellipsis}
    .w-members .st{flex-shrink:0;margin-left:auto;width:7px;height:7px;border-radius:50%}
    .w-members .st.on{background:var(--mx-success)}
    .w-members .st.busy{background:var(--mx-warning)}
    .w-members .st.off{background:color-mix(in currentColor,30%,transparent)}"""


def color(tone):
    return TONES.get(tone) or "currentColor"


def tone_bg(tone):
    t = TONES.get(tone)
    return f"color-mix(in srgb,{t} 12%,transparent)" if t else "color-mix(in currentColor,12%,transparent)"


def esc(s):
    return str(s).replace("&", "&amp;").replace("<", "&lt;").replace(">", "&gt;").replace('"', "&quot;")


def card_of(c, inner, widget=None):
    title = f' title="{esc(c["title"])}"' if c.get("title") else ""
    h = f' h="{int(c["h"])}"' if c.get("h") else ""
    classes = (["w-" + widget] if widget else []) + (c.get("class") or "").split()
    cls = f' class="{" ".join(classes)}"' if classes else ""
    return f'  <card id="{esc(c.get("id", "card"))}" span="{int(c.get("span", 6))}"{h}{title}{cls}>\n{inner}\n  </card>'


def w_stat(c):
    n_style = f' style="color:{color(c.get("tone"))}"' if c.get("tone", "neutral") != "neutral" else ""
    sub = f'<span class="d">{esc(c["sub"])}</span>' if c.get("sub") else ""
    return card_of(c, f'<div class="s"><span class="n"{n_style}>{esc(c.get("value", ""))}</span>{sub}</div>', "stat")


def w_progress(c):
    pct = max(0, min(100, float(c.get("percent", 0))))
    marks = "".join(
        f'<span class="mark" style="left:{float(m["left"])}%"></span>' for m in c.get("marks", []))
    label = f'<div style="font-size:13px;margin-bottom:6px">{esc(c["label"])}</div>' if c.get("label") else ""
    return card_of(c, f'{label}\n    <div class="bar"><i style="width:{pct}%;background:{color(c.get("tone", "ok"))}"></i>{marks}</div>', "progress")


def w_bars(c):
    rows = c.get("rows", [])
    top = max((float(r.get("value", 0)) for r in rows), default=1) or 1
    out = []
    for r in rows:
        pct = round(float(r.get("value", 0)) / top * 100, 1)
        out.append(f'    <div class="row"><span class="name">{esc(r.get("name", ""))}</span>'
                   f'<span class="track"><i style="width:{pct}%;background:{color(r.get("tone"))}"></i></span>'
                   f'<span class="v">{esc(r.get("value", ""))}</span></div>')
    return card_of(c, "\n".join(out), "bars")


def w_funnel(c):
    steps = c.get("steps", [])
    base = float(steps[0].get("value", 1)) if steps else 1
    last = len(steps) - 1
    out = []
    for i, s in enumerate(steps):
        pct = round(float(s.get("value", 0)) / base * 100, 1) if base else 0
        bg = "var(--mx-success)" if i == last and last > 0 else "var(--mx-accent)"
        out.append(f'    <div class="step"><span class="label">{esc(s.get("label", ""))}</span>'
                   f'<span class="bar" style="width:{max(pct, 4)}%;background:{bg}">{esc(s.get("value", ""))}</span></div>')
    return card_of(c, '<div class="f">\n' + "\n".join(out) + "\n    </div>", "funnel")


def w_flow(c):
    cols = []
    for col in c.get("columns", []):
        items = []
        for it in col.get("items", []):
            tag = (f'<span class="tag" style="background:{tone_bg(it.get("tone", "neutral"))};'
                   f'color:{color(it.get("tone"))}">{esc(it["tag"])}</span>') if it.get("tag") else ""
            meta = f'<span class="meta">{tag}{esc(it.get("meta") or "")}</span>' if (tag or it.get("meta")) else ""
            items.append(f'      <div class="t">{esc(it.get("text", ""))}{meta}</div>')
        cols.append(f'    <div class="col"><h4>{esc(col.get("name", ""))} <b>{len(col.get("items", []))}</b></h4>\n'
                    + "\n".join(items) + "\n    </div>")
    return card_of(c, '<div class="cols">\n' + "\n".join(cols) + "\n    </div>", "flow")


def w_table(c):
    heads = "".join(f"<th>{esc(h)}</th>" for h in c.get("columns", []))
    rows = []
    for row in c.get("rows", []):
        cells = []
        for cell in row:
            if isinstance(cell, dict):
                cells.append(f'<td><span class="tag" style="background:{tone_bg(cell.get("tone", "neutral"))};'
                             f'color:{color(cell.get("tone"))}">{esc(cell.get("text", ""))}</span></td>')
            else:
                cells.append(f"<td>{esc(cell)}</td>")
        rows.append("    <tr>" + "".join(cells) + "</tr>")
    return card_of(c, f'    <table>\n    <tr>{heads}</tr>\n' + "\n".join(rows) + "\n    </table>", "table")


def w_timeline(c):
    out = []
    for it in c.get("items", []):
        cls = {"done": "m done", "doing": "m doing"}.get(it.get("state", "todo"), "m")
        date = f'<span class="d">{esc(it["date"])}</span>' if it.get("date") else ""
        out.append(f'    <div class="{cls}">{esc(it.get("text", ""))}{date}</div>')
    return card_of(c, '<div class="tl">\n' + "\n".join(out) + "\n    </div>", "timeline")


def w_gantt(c):
    days = c.get("days", [])
    n = len(days) or 1
    today = c.get("today")
    head = "".join(f'<span class="h">{esc(d)}</span>' for d in days)
    out = []
    for row in c.get("rows", []):
        bars = "".join(
            f'<i class="bar" style="left:{float(b["from"]) / n * 100}%;width:{(float(b["to"]) - float(b["from"]) + 1) / n * 100}%;background:{color(b.get("tone", "accent"))}">{esc(b.get("label", ""))}</i>'
            for b in row.get("bars", []))
        line = '<span class="today"></span>' if today is not None else ""
        out.append(f'    <span class="name">{esc(row.get("name", ""))}</span>\n'
                   f'    <span class="track">{bars}{line}</span>')
    # 栅格列数 = 天数（实例相关），样式留卡内；颜色仍走 token
    return card_of(c, f'''    <style>.g{{display:grid;grid-template-columns:72px repeat({n},1fr);font-size:12px;row-gap:6px;align-items:center}}.g .h{{color:var(--mx-text-tertiary);text-align:center;font-size:10.5px;white-space:nowrap;overflow:hidden}}.g .name{{color:var(--mx-text-secondary);white-space:nowrap;overflow:hidden;text-overflow:ellipsis;padding-right:8px}}.g .track{{grid-column:2 / span {n};position:relative;height:14px;border-radius:4px;background:color-mix(in currentColor,6%,transparent)}}.g .bar{{position:absolute;top:2px;bottom:2px;border-radius:4px;color:{ON_ACCENT};font-size:10px;line-height:10px;padding:0 6px;white-space:nowrap;overflow:hidden}}.g .today{{position:absolute;top:-2px;bottom:-2px;width:1.5px;background:var(--mx-danger)}}</style>
    <div class="g">
    <span></span>{head}
''' + "\n".join(out) + "\n    </div>", "gantt")


def w_members(c):
    out = []
    for i, p in enumerate(c.get("people", [])):
        st = {"on": "on", "busy": "busy", "off": "off"}.get(p.get("state", "on"), "on")
        initial = (p.get("name") or "?")[0]
        ava = p.get("color") or AVA_COLORS[i % len(AVA_COLORS)]
        out.append(f'    <div class="m"><span class="ava" style="background:{ava}">{esc(initial)}</span>'
                   f'<span class="who"><b>{esc(p.get("name", ""))}</b><span>{esc(p.get("role", ""))}</span></span>'
                   f'<span class="st {st}"></span></div>')
    return card_of(c, '<div class="grid">\n' + "\n".join(out) + "\n    </div>", "members")


def w_raw(c):
    if c.get("html_file"):
        with open(c["html_file"], encoding="utf-8") as f:
            return card_of(c, f.read())
    return card_of(c, c.get("html", ""))


WIDGETS = {"stat": w_stat, "progress": w_progress, "bars": w_bars, "funnel": w_funnel,
           "flow": w_flow, "table": w_table, "timeline": w_timeline, "gantt": w_gantt,
           "members": w_members, "raw": w_raw}


def main():
    args = sys.argv[1:]
    if "--help" in args or not args:
        print(__doc__)
        return
    src = args[0]
    out_path = None
    if "-o" in args:
        out_path = args[args.index("-o") + 1]
    if src == "-":
        spec = json.load(sys.stdin)
    else:
        with open(src, encoding="utf-8") as f:
            spec = json.load(f)
    board = spec.get("board", {})
    gap = int(board.get("gap", 12))
    board_cls = (board.get("class") or "").strip()
    cls_attr = f' class="{esc(board_cls)}"' if board_cls else ""
    parts = [f'<board name="{esc(board.get("name", "仪表板"))}" gap="{gap}"{cls_attr}>']
    parts.append(f'  <style>\n{BOARD_CSS}\n  </style>')
    if (board.get("style") or "").strip():
        parts.append(f'  <style>\n{board["style"].strip()}\n  </style>')
    for c in spec.get("cards", []):
        w = c.get("widget", "raw")
        if w not in WIDGETS:
            raise SystemExit(f"未知 widget：{w}（可用：{'/'.join(WIDGETS)}）")
        parts.append(WIDGETS[w](c))
    parts.append("</board>")
    text = "\n".join(parts) + "\n"
    if out_path:
        with open(out_path, "w", encoding="utf-8") as f:
            f.write(text)
        print(f"已生成 {out_path}（{len(spec.get('cards', []))} 张卡）")
    else:
        sys.stdout.write(text)


if __name__ == "__main__":
    main()
