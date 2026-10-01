#!/usr/bin/env python3
"""遮罩生成：alpha 阈值化 → 手工构造 P4 PBM（规避 PIL 写 PBM 的位反相问题）。

用法: python3 make_mask.py <源图> <输出.pbm> [--threshold 128]
内置对账：重新解析 PBM 统计黑位数，必须与形状像素数一致，否则非零退出。
"""
import argparse
import base64
import io
import re
import sys

import numpy as np
from PIL import Image


def load_alpha(path: str, threshold: int):
    """提取 alpha 通道并阈值化为布尔遮罩（True=图形）。"""
    if path.lower().endswith('.svg'):
        svg = open(path, encoding='utf-8').read()
        m = re.search(r'xlink:href="data:image/(png|jpeg);base64,([^"]+)"', svg)
        if not m:
            sys.exit('错误: SVG 中未找到内嵌位图')
        img = Image.open(io.BytesIO(base64.b64decode(m.group(2)))).convert('RGBA')
    else:
        img = Image.open(path).convert('RGBA')
    alpha = img.split()[3].point(lambda v: 255 if v >= threshold else 0)
    return img.size, np.array(alpha, dtype=np.uint8) >= threshold


def main() -> None:
    ap = argparse.ArgumentParser()
    ap.add_argument('source', help='源图路径（png/jpg/svg）')
    ap.add_argument('output', help='输出 PBM 路径')
    ap.add_argument('--threshold', type=int, default=128, help='alpha 阈值（默认 128）')
    args = ap.parse_args()

    (w, h), arr = load_alpha(args.source, args.threshold)
    pad = (-w) % 8
    if pad:
        arr = np.pad(arr, ((0, 0), (0, pad)))
    with open(args.output, 'wb') as f:
        f.write(b'P4\n%d %d\n' % (w, h))
        f.write(np.packbits(arr, axis=1).tobytes())

    # 对账：重新解析 PBM（头两行：P4 与 W H），黑位数必须等于形状像素数
    data = open(args.output, 'rb').read()
    first = data.index(b'\n')
    hdr_end = data.index(b'\n', first + 1) + 1
    body = data[hdr_end:]
    black = sum(bin(b).count('1') for b in body)
    shape = int(arr.sum())
    if black != shape:
        sys.exit(f'错误: PBM 对账失败（黑位 {black} != 形状像素 {shape}）')
    print(f'PBM 已生成: {args.output}（{w}x{h}，黑位 {black}，对账通过）')


if __name__ == '__main__':
    main()
