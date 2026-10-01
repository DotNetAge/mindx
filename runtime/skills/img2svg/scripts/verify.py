#!/usr/bin/env python3
"""验证：Playwright 真实渲染矢量 SVG，与源图形状逐像素比对 IoU（≥0.97 通过）。

用法: python3 verify.py <源图> <矢量.svg> [更多矢量.svg ...]
自动按 fill 色选择判定阈值（黑形白底 <128 / 白形深底 ≥200，深底自动注入）。
Chromium 路径自动发现：环境变量 CHROMIUM_PATH 优先，否则探测 Playwright 缓存目录。
"""
import argparse
import base64
import glob
import io
import os
import re
import sys

from PIL import Image
from playwright.sync_api import sync_playwright


def find_chromium() -> str:
    """定位 Chromium 可执行文件：CHROMIUM_PATH 环境变量 > Playwright 缓存探测。"""
    env = os.environ.get('CHROMIUM_PATH')
    if env and os.path.exists(env):
        return env
    home = os.path.expanduser('~')
    patterns = [
        home + '/Library/Caches/ms-playwright/chromium-*/chrome-mac*/'
        'Google Chrome for Testing.app/Contents/MacOS/Google Chrome for Testing',
        home + '/Library/Caches/ms-playwright/chromium-*/chrome-mac*/'
        'Chromium.app/Contents/MacOS/Chromium',
        home + '/.cache/ms-playwright/chromium-*/chrome-linux*/chrome',
    ]
    for pat in patterns:
        hits = sorted(glob.glob(pat), reverse=True)
        if hits:
            return hits[0]
    sys.exit('错误: 未找到 Chromium，请先执行 playwright install chromium，'
             '或用环境变量 CHROMIUM_PATH 指定可执行文件路径')


def load_shape(path: str) -> set:
    """源图 alpha≥128 的形状像素集合（真值）。"""
    if path.lower().endswith('.svg'):
        svg = open(path, encoding='utf-8').read()
        m = re.search(r'xlink:href="data:image/(png|jpeg);base64,([^"]+)"', svg)
        if not m:
            sys.exit('错误: SVG 中未找到内嵌位图')
        img = Image.open(io.BytesIO(base64.b64decode(m.group(2)))).convert('RGBA')
    else:
        img = Image.open(path).convert('RGBA')
    px = img.load()
    return img.size, {(x, y) for y in range(img.height)
                      for x in range(img.width) if px[x, y][3] >= 128}


def main() -> None:
    ap = argparse.ArgumentParser()
    ap.add_argument('source', help='源图（真值基准）')
    ap.add_argument('vectors', nargs='+', help='待验证的矢量 SVG')
    args = ap.parse_args()

    (w, h), truth = load_shape(args.source)

    with sync_playwright() as p:
        browser = p.chromium.launch(headless=True, executable_path=find_chromium())
        page = browser.new_page(viewport={'width': w, 'height': h})
        failed = []
        for vec in args.vectors:
            svg_text = open(vec, encoding='utf-8').read()
            is_white = "fill=\"#FFFFFF\"" in svg_text or "fill='white'" in svg_text
            page.goto('file://' + vec)
            if is_white:
                # 白形需深底衬托：直接在 SVG 文档根节点设背景色
                page.evaluate(
                    "document.documentElement.style.background='#202226'")
                page.wait_for_timeout(100)
            shot = f'/tmp/verify-{vec.rsplit("/", 1)[-1]}.png'
            page.screenshot(path=shot)
            sp = Image.open(shot).convert('L').load()
            if is_white:
                got = {(x, y) for y in range(h) for x in range(w)
                       if sp[x, y] >= 200}
            else:
                got = {(x, y) for y in range(h) for x in range(w)
                       if sp[x, y] < 128}
            inter = len(truth & got)
            union = len(truth | got)
            iou = inter / union if union else 0.0
            ok = iou >= 0.97
            print(f'{vec.rsplit("/", 1)[-1]}: IoU={iou:.4f} '
                  f'真值={len(truth)} 渲染={len(got)} {"通过" if ok else "偏差过大"}')
            if not ok:
                failed.append(vec)
        browser.close()
    if failed:
        sys.exit(f'验证未通过: {", ".join(failed)}')


if __name__ == '__main__':
    main()
