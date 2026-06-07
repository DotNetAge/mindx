name: mindx
base: core24
version: __VERSION__
summary: AI-native multi-agent conversation platform
description: |
  MindX is an AI-native multi-agent conversation platform with OPC capabilities.
  Manage AI agents, providers, and conversations from your terminal or web UI.

confinement: strict
grade: stable

apps:
  mindx:
    command: bin/mindx
    completer: bash-completion/mindx
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
      CGO_ENABLED=0 go build \
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
