name: mindx
base: core24
version: __VERSION__
summary: AI-native multi-agent conversation platform
description: |
  MindX is an AI-native multi-agent conversation platform with OPC capabilities.
  Manage AI agents, providers, and conversations from your terminal or web UI.

title: MindX
contact: ray@dotnetage.com
license: MIT
issues: https://github.com/DotNetAge/mindx/issues
source-code: https://github.com/DotNetAge/mindx
website: https://github.com/DotNetAge/mindx

confinement: strict
grade: stable

apps:
  mindx:
    command: bin/mindx
    plugs:
      - network
      - network-bind
      - home
      - removable-media

  daemon:
    command: bin/mindx daemon
    daemon: simple
    restart-condition: always
    plugs:
      - network
      - network-bind
      - home
      - removable-media

parts:
  mindx:
    plugin: go
    source-type: local
    source: .
    build-snaps:
      - go/latest/stable
    override-build: |
      cd $CRAFT_PART_SRC_WORK
      CGO_ENABLED=1 go build \
        -trimpath \
        -ldflags="-s -w -X github.com/DotNetAge/mindx/cmd.Version=__VERSION__" \
        -o $CRAFT_PART_INSTALL/bin/mindx .
    build-packages:
      - gcc

  runtime:
    plugin: dump
    source: runtime/
    organize:
      '*': usr/share/mindx/runtime/
