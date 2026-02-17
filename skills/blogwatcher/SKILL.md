---
name: blogwatcher
description: 博客监控技能，跟踪博客更新、RSS订阅和文章变化
version: 1.0.0
category: general
tags:
  - blog
  - rss
  - feed
  - monitor
  - 博客
  - RSS订阅
  - 文章更新
  - 博客监控
os:
  - darwin
  - linux
enabled: true
timeout: 60
command: blogwatcher
requires:
  bins:
    - blogwatcher
homepage: https://github.com/Hyaxia/blogwatcher
---

# 博客监控技能

使用 `blogwatcher` CLI 跟踪博客和 RSS/Atom 源的更新。

## 安装

```bash
go install github.com/Hyaxia/blogwatcher/cmd/blogwatcher@latest
```

## 常用命令

- 添加博客: `blogwatcher add "我的博客" https://example.com`
- 列出博客: `blogwatcher blogs`
- 扫描更新: `blogwatcher scan`
- 列出文章: `blogwatcher articles`
- 标记文章已读: `blogwatcher read 1`
- 标记所有文章已读: `blogwatcher read-all`
- 移除博客: `blogwatcher remove "我的博客"`

## 示例输出

```
$ blogwatcher blogs
Tracked blogs (1):

  xkcd
    URL: https://xkcd.com
```

```
$ blogwatcher scan
Scanning 1 blog(s)...

  xkcd
    Source: RSS | Found: 4 | New: 4

Found 4 new article(s) total!
```

## 注意事项

- 使用 `blogwatcher <command> --help` 来发现标志和选项
