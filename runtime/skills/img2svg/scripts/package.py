#!/usr/bin/env python3
"""包装：potrace 输出 → 规范 SVG（一条 path 多色版本，黑白共用同一条轮廓）。

用法: python3 package.py <trace.svg> <宽> <高> <输出前缀> [--colors black,white]
生成: <前缀>-mono.svg（黑）/ <前缀>-white.svg（白），颜色映射 black→#000000、white→#FFFFFF。
"""
import argparse
import re
import sys

FILLS = {'black': '#000000', 'white': '#FFFFFF'}
LABELS = {'black': '黑白版', 'white': '纯白版'}
SUFFIX = {'black': 'mono', 'white': 'white'}  # 文件名后缀（black→mono 约定）


def main() -> None:
    ap = argparse.ArgumentParser()
    ap.add_argument('trace', help='potrace 输出的 SVG')
    ap.add_argument('width', type=int, help='原位图宽（viewBox 用）')
    ap.add_argument('height', type=int, help='原位图高（viewBox 用）')
    ap.add_argument('prefix', help='输出文件前缀，如 assets/mindx')
    ap.add_argument('--colors', default='black,white', help='逗号分隔：black,white')
    args = ap.parse_args()

    trace = open(args.trace, encoding='utf-8').read()
    paths = re.findall(r'<path d="([^"]+)"', trace, re.S)
    if not paths:
        sys.exit('错误: trace.svg 中未找到 path（描摹可能失败）')
    transform_m = re.search(r'<g[^>]*transform="([^"]+)"', trace)
    if not transform_m:
        sys.exit('错误: trace.svg 中未找到 transform（坐标映射缺失）')
    d = ' '.join(paths)

    for name in args.colors.split(','):
        name = name.strip()
        if name not in FILLS:
            sys.exit(f'错误: 未知颜色 {name}（可选 black/white）')
        out = (f'<?xml version="1.0" encoding="UTF-8"?>\n'
               f'<svg width="{args.width}px" height="{args.height}px" '
               f'viewBox="0 0 {args.width} {args.height}" '
               f'xmlns="http://www.w3.org/2000/svg">\n'
               f'  <title>MindX 图标（{LABELS[name]}）</title>\n'
               f'  <g transform="{transform_m.group(1)}">'
               f'<path d="{d}" fill="{FILLS[name]}"/></g>\n</svg>\n')
        dst = f'{args.prefix}-{SUFFIX[name]}.svg'
        open(dst, 'w', encoding='utf-8').write(out)
        print(f'已生成: {dst}（{len(out) // 1024 or 1} KB）')


if __name__ == '__main__':
    main()
