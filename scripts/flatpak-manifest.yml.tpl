app-id: com.dotnetage.mindx
runtime: org.freedesktop.Platform//24.08
runtime-version: '24.08'
sdk: org.freedesktop.Sdk//24.08'
command: mindx

finish-args:
  # Network access
  --share=network
  # File system access
  --filesystem=home
  --filesystem=host
  # D-Bus (for desktop integration)
  --talk-name=org.freedesktop.DBus
  --talk-name=org.freedesktop.Notifications
  # D-Bus (for daemon activation)
  --own-name=com.dotnetage.MindX.Daemon

modules:
  - name: mindx
    buildsystem: simple
    build-commands:
      - install -Dm755 mindx /app/bin/mindx
      - install -Dm644 scripts/com.dotnetage.mindx.desktop /app/share/applications/com.dotnetage.mindx.desktop
      - install -Dm644 scripts/com.dotnetage.mindx.svg /app/share/icons/hicolor/scalable/apps/com.dotnetage.mindx.svg
      - install -Dm644 scripts/com.dotnetage.MindX.Daemon.service /app/share/dbus-1/services/com.dotnetage.MindX.Daemon.service
    sources:
      - type: archive
        url: __RELEASE_URL__
        sha256: __SHA256_AMD64__
        dest-filename: mindx-linux-amd64.tar.gz
      - type: file
        path: scripts/com.dotnetage.mindx.desktop
      - type: file
        path: scripts/com.dotnetage.mindx.svg
      - type: file
        path: scripts/com.dotnetage.MindX.Daemon.service
