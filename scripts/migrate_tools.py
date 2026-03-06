#!/usr/bin/env python3
"""
工具迁移脚本
从 skills/ 目录中提取工具到独立的 tools/ 目录
"""

import os
import json
import shutil
import yaml
from pathlib import Path
from typing import Dict, List

class ToolMigrator:
    def __init__(self, skills_dir: str, tools_dir: str, dry_run: bool = False):
        self.skills_dir = Path(skills_dir)
        self.tools_dir = Path(tools_dir)
        self.dry_run = dry_run
        self.migrated = []
        self.skipped = []
        self.errors = []

    def has_tool_files(self, skill_dir: Path) -> bool:
        """检查是否有工具文件"""
        tool_extensions = ['.py', '.sh', '.go', '.js']
        for ext in tool_extensions:
            if list(skill_dir.glob(f'*{ext}')):
                return True
        return False

    def extract_tool_info(self, skill_dir: Path) -> Dict:
        """从 SKILL.md 提取工具信息"""
        skill_md = skill_dir / 'SKILL.md'
        if not skill_md.exists():
            return None

        with open(skill_md, 'r', encoding='utf-8') as f:
            content = f.read()

        # 解析 YAML frontmatter
        if not content.startswith('---'):
            return None

        parts = content.split('---', 2)
        if len(parts) < 3:
            return None

        try:
            metadata = yaml.safe_load(parts[1])
            return metadata
        except:
            return None

    def create_tool_json(self, skill_name: str, metadata: Dict, tool_file: Path) -> Dict:
        """创建 tool.json 配置"""
        # 确定工具类型
        tool_type = 'shell'
        if tool_file.suffix == '.py':
            tool_type = 'python'
        elif tool_file.suffix == '.go':
            tool_type = 'go'
        elif tool_file.suffix == '.js':
            tool_type = 'shell'  # Node.js 也用 shell 执行

        tool_config = {
            'name': skill_name,
            'description': metadata.get('description', f'{skill_name} 工具'),
            'version': metadata.get('version', '1.0.0'),
            'type': tool_type,
            'command': tool_file.name,
            'parameters': metadata.get('parameters', {}),
            'timeout': metadata.get('timeout', 30)
        }

        return tool_config

    def migrate_tool(self, skill_dir: Path):
        """迁移单个工具"""
        skill_name = skill_dir.name

        print(f"[{skill_name}] 检查...")

        # 1. 检查是否有工具文件
        if not self.has_tool_files(skill_dir):
            self.skipped.append(f"{skill_name}: 没有工具文件")
            print(f"  ⊘ 跳过: 没有工具文件")
            return

        # 2. 提取工具信息
        metadata = self.extract_tool_info(skill_dir)
        if not metadata:
            self.errors.append(f"{skill_name}: 无法解析 SKILL.md")
            print(f"  ✗ 错误: 无法解析 SKILL.md")
            return

        # 3. 查找工具文件
        tool_files = []
        for ext in ['.py', '.sh', '.go', '.js']:
            tool_files.extend(skill_dir.glob(f'*{ext}'))

        if not tool_files:
            self.skipped.append(f"{skill_name}: 没有找到工具文件")
            print(f"  ⊘ 跳过: 没有找到工具文件")
            return

        # 使用第一个工具文件
        tool_file = tool_files[0]

        # 4. 创建目标目录
        target_dir = self.tools_dir / skill_name

        if not self.dry_run:
            target_dir.mkdir(parents=True, exist_ok=True)

            # 5. 复制工具文件
            shutil.copy2(tool_file, target_dir / tool_file.name)

            # 6. 创建 tool.json
            tool_config = self.create_tool_json(skill_name, metadata, tool_file)
            with open(target_dir / 'tool.json', 'w', encoding='utf-8') as f:
                json.dump(tool_config, f, indent=2, ensure_ascii=False)

            print(f"  ✓ 成功: {tool_file.name} -> tools/{skill_name}/")
        else:
            print(f"  → 将迁移: {tool_file.name} -> tools/{skill_name}/")

        self.migrated.append(skill_name)

    def migrate_all(self):
        """迁移所有工具"""
        print(f"{'='*60}")
        print(f"工具迁移脚本")
        print(f"{'='*60}")
        print(f"源目录: {self.skills_dir}")
        print(f"目标目录: {self.tools_dir}")
        print(f"模式: {'DRY RUN' if self.dry_run else 'EXECUTE'}")
        print(f"{'='*60}")
        print()

        # 创建 tools 目录
        if not self.dry_run:
            self.tools_dir.mkdir(parents=True, exist_ok=True)

        # 遍历 skills 目录
        for skill_dir in sorted(self.skills_dir.iterdir()):
            if not skill_dir.is_dir():
                continue
            if skill_dir.name.startswith('.'):
                continue

            self.migrate_tool(skill_dir)

        # 打印总结
        print()
        print(f"{'='*60}")
        print(f"迁移总结")
        print(f"{'='*60}")
        print(f"✓ 成功迁移: {len(self.migrated)} 个工具")
        print(f"⊘ 跳过: {len(self.skipped)} 个")
        print(f"✗ 错误: {len(self.errors)} 个")
        print(f"{'='*60}")

        if self.migrated:
            print()
            print("成功迁移的工具:")
            for tool in self.migrated:
                print(f"  - {tool}")

        if self.skipped:
            print()
            print("跳过的 skills:")
            for item in self.skipped:
                print(f"  - {item}")

        if self.errors:
            print()
            print("错误:")
            for item in self.errors:
                print(f"  - {item}")

def main():
    import argparse

    parser = argparse.ArgumentParser(description='迁移工具到独立目录')
    parser.add_argument('--skills-dir', default='skills', help='Skills 目录')
    parser.add_argument('--tools-dir', default='tools', help='Tools 目录')
    parser.add_argument('--dry-run', action='store_true', help='只检查，不实际迁移')

    args = parser.parse_args()

    migrator = ToolMigrator(args.skills_dir, args.tools_dir, args.dry_run)
    migrator.migrate_all()

if __name__ == '__main__':
    main()
