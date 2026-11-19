#!/usr/bin/env python3
"""把市场仓的头像库存同步进 mindx/runtime/assets，统一 256×256 JPEG。

用法（在 mindx 仓根目录执行）：
    python3 scripts/sync-avatar-library.py [--src <market 仓库根>] [--dry-run]

源：<market>/assets/avatars/*（成员头像）、<market>/assets/teams/*（团队头像）
目标：runtime/assets/avatars/*.jpg、runtime/assets/teams/*.jpg

规则：
- 一律按 256×256 居中裁切再缩放（原图 1024/2048，直接拉伸会变形）。
- 编码 JPEG q85，去掉原图的体积与动画开销；同文件名改扩展名 .jpg。
- 目标目录整体重建（这份库是派生资产，源改了就该整体刷新，不做增量猜测）。
- 体积超 8MB 时在输出里告警——内置进 daemon 二进制要考虑安装包体积。
"""
import argparse
import pathlib
import shutil
import sys

from PIL import Image

TARGET = 256
QUALITY = 85
BUDGET_MB = 8


def convert(src_dir: pathlib.Path, dst_dir: pathlib.Path) -> int:
    dst_dir.mkdir(parents=True, exist_ok=True)
    for f in dst_dir.glob('*.jpg'):
        f.unlink()
    count = 0
    for src in sorted(src_dir.iterdir()):
        if not src.is_file():
            continue
        if src.suffix.lower() not in ('.png', '.jpg', '.jpeg', '.webp'):
            continue
        try:
            im = Image.open(src)
        except Exception as e:
            print(f'跳过（无法打开）{src.name}: {e}', file=sys.stderr)
            continue
        im = im.convert('RGB')
        side = min(im.size)
        left = (im.width - side) // 2
        top = (im.height - side) // 2
        im = im.crop((left, top, left + side, top + side)).resize((TARGET, TARGET), Image.LANCZOS)
        im.save(dst_dir / (src.stem + '.jpg'), 'JPEG', quality=QUALITY, optimize=True)
        count += 1
    return count


def dir_size(path: pathlib.Path) -> int:
    return sum(f.stat().st_size for f in path.rglob('*') if f.is_file())


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument('--src', default='../mindx-market', help='market 仓库根（含 assets/）')
    ap.add_argument('--dry-run', action='store_true', help='只统计源数量，不写盘')
    args = ap.parse_args()

    repo = pathlib.Path(__file__).resolve().parents[1]
    src_root = (repo / args.src).resolve() if not pathlib.Path(args.src).is_absolute() else pathlib.Path(args.src)
    if not src_root.is_dir():
        print(f'源仓库不存在：{src_root}', file=sys.stderr)
        return 3

    plan = []
    for group in ('avatars', 'teams'):
        s = src_root / 'assets' / group
        if not s.is_dir():
            print(f'缺目录：{s}', file=sys.stderr)
            return 3
        plan.append((group, s, repo / 'runtime' / 'assets' / group))

    if args.dry_run:
        for group, s, _ in plan:
            n = len([p for p in s.iterdir() if p.is_file()])
            print(f'{group}: {n} 张源图')
        return 0

    total = 0
    for group, s, d in plan:
        n = convert(s, d)
        total += n
        print(f'{group}: {n} 张 → {d.relative_to(repo)}')

    size = dir_size(repo / 'runtime' / 'assets')
    mb = size / 1024 / 1024
    print(f'合计 {total} 张，{mb:.1f} MB')
    if mb > BUDGET_MB:
        print(f'告警：超过 {BUDGET_MB}MB 预算，考虑降低质量或裁剪数量', file=sys.stderr)
    return 0


if __name__ == '__main__':
    sys.exit(main())
