#!/usr/bin/env python3
"""源图分析：读取 PNG/JPG 或内嵌位图的 SVG，输出结构数据与目视预览。

用法: python3 analyze.py <源图> [--preview <预览输出.png>]
输出: 尺寸、alpha 分布（判断抗锯齿）、形状像素数、亮度中位（判断底色取向）。
"""
import argparse
import base64
import io
import re
import sys

from PIL import Image
from collections import Counter


def load_image(path: str) -> Image.Image:
    """加载源图：SVG 自动提取内嵌 base64 位图，其余交给 PIL。"""
    if path.lower().endswith('.svg'):
        svg = open(path, encoding='utf-8').read()
        m = re.search(r'xlink:href="data:image/(png|jpeg);base64,([^"]+)"', svg)
        if not m:
            sys.exit('错误: SVG 中未找到内嵌位图，本脚本仅处理位图型 SVG')
        return Image.open(io.BytesIO(base64.b64decode(m.group(2)))).convert('RGBA')
    return Image.open(path).convert('RGBA')


def main() -> None:
    ap = argparse.ArgumentParser()
    ap.add_argument('source', help='源图路径（png/jpg/svg）')
    ap.add_argument('--preview', help='可选：输出白底合成预览 PNG 供目视')
    args = ap.parse_args()

    img = load_image(args.source)
    w, h = img.size
    px = list(img.getdata())

    alpha_hist = Counter(p[3] for p in px)
    opaque = sum(n for a, n in alpha_hist.items() if a >= 128)
    binary = alpha_hist.get(0, 0) + alpha_hist.get(255, 0)
    lums = sorted(l for l, p in zip(
        img.convert('L').getdata(), px) if p[3] > 128)
    median = lums[len(lums) // 2] if lums else -1

    print(f'尺寸: {w}x{h}')
    print(f'alpha 取值档数: {len(alpha_hist)}（仅 0/255 两档 = 无抗锯齿）')
    print(f'形状像素（alpha≥128）: {opaque}')
    print(f'二值像素占比: {binary / len(px):.2%}')
    print(f'不透明像素亮度中位: {median}（<128 深色图形，≥128 浅色图形）')
    if args.preview:
        bg = Image.new('RGBA', img.size, (255, 255, 255, 255))
        bg.alpha_composite(img)
        bg.convert('RGB').save(args.preview)
        print(f'预览已输出: {args.preview}')


if __name__ == '__main__':
    main()
