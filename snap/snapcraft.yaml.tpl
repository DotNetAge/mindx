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

environment:
  LD_LIBRARY_PATH: ${LD_LIBRARY_PATH}:${SNAP}/usr/local/lib

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

  onnxruntime:
    plugin: nil
    build-packages:
      - curl
      - ca-certificates
    override-build: |
      ONNX_VERSION="1.24.4"
      case "${CRAFT_ARCH_BUILD_ON}" in
        amd64|x86_64) ARCH="x64" ;;
        arm64|aarch64) ARCH="aarch64" ;;
        *) echo "Unsupported arch: ${CRAFT_ARCH_BUILD_ON}"; exit 1 ;;
      esac
      curl -fL -o /tmp/onnxruntime.tgz \
        "https://github.com/microsoft/onnxruntime/releases/download/v${ONNX_VERSION}/onnxruntime-linux-${ARCH}-${ONNX_VERSION}.tgz"
      tar xzf /tmp/onnxruntime.tgz -C /tmp
      mkdir -p "${CRAFT_PART_INSTALL}/usr/local/lib"
      cp -P "/tmp/onnxruntime-linux-${ARCH}-${ONNX_VERSION}/lib/libonnxruntime.so"* "${CRAFT_PART_INSTALL}/usr/local/lib/"
      rm -rf /tmp/onnxruntime*
    prime:
      - usr/local/lib/libonnxruntime.so*
