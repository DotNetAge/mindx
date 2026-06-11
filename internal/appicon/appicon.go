package appicon

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
)

const (
	Name     = "Mindx"
	ID       = "com.dotnetage.mindx"
	Version  = "1.0.0"
	IconName = "mindx.png"
)

// IconPath is the path of the icon inside the embedded FS.
const IconPath = "assets/images/" + IconName

// Bytes reads the app icon PNG from the embedded filesystem.
func Bytes(iconFS fs.FS) ([]byte, error) {
	return fs.ReadFile(iconFS, IconPath)
}

// Write extracts and writes the embedded icon to destPath.
func Write(iconFS fs.FS, destPath string) error {
	data, err := Bytes(iconFS)
	if err != nil {
		return fmt.Errorf("read embedded icon: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return fmt.Errorf("create icon dir: %w", err)
	}
	return os.WriteFile(destPath, data, 0o644)
}

// CreateAppBundle generates a macOS .app bundle at outputDir.
//
// The bundle wraps binaryPath with a proper Info.plist and the embedded
// icon, so Finder / Dock display the custom MindX icon.
//
// Example:
//
//	appicon.CreateAppBundle(iconFS, "/Applications", "/path/to/mindx")
//	// → creates /Applications/Mindx.app/Contents/...
func CreateAppBundle(iconFS fs.FS, outputDir, binaryPath string) (string, error) {
	if runtime.GOOS != "darwin" {
		return "", fmt.Errorf("CreateAppBundle only supported on macOS")
	}

	binAbs, err := filepath.Abs(binaryPath)
	if err != nil {
		return "", fmt.Errorf("resolve binary path: %w", err)
	}

	appDir := filepath.Join(outputDir, Name+".app")
	contentsDir := filepath.Join(appDir, "Contents")
	macosDir := filepath.Join(contentsDir, "MacOS")
	resourcesDir := filepath.Join(contentsDir, "Resources")

	for _, dir := range []string{contentsDir, macosDir, resourcesDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return "", fmt.Errorf("create %s: %w", dir, err)
		}
	}

	iconDest := filepath.Join(resourcesDir, IconName)
	if err := Write(iconFS, iconDest); err != nil {
		return "", fmt.Errorf("write icon: %w", err)
	}

	execDest := filepath.Join(macosDir, "mindx")
	if err := os.Symlink(binAbs, execDest); err != nil {
		data, rErr := os.ReadFile(binAbs)
		if rErr != nil {
			return "", fmt.Errorf("read binary for copy: %w", rErr)
		}
		if wErr := os.WriteFile(execDest, data, 0o755); wErr != nil {
			return "", fmt.Errorf("write binary: %w", wErr)
		}
	}

	plistContent := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleExecutable</key>
	<string>mindx</string>
	<key>CFBundleIdentifier</key>
	<string>%s</string>
	<key>CFBundleInfoDictionaryVersion</key>
	<string>6.0</string>
	<key>CFBundleName</key>
	<string>%s</string>
	<key>CFBundleDisplayName</key>
	<string>%s</string>
	<key>CFBundlePackageType</key>
	<string>APPL</string>
	<key>CFBundleShortVersionString</key>
	<string>%s</string>
	<key>CFBundleVersion</key>
	<string>%s</string>
	<key>CFBundleIconFile</key>
	<string>%s</string>
	<key>LSMinimumSystemVersion</key>
	<string>12.0</string>
	<key>NSHighResolutionCapable</key>
	<true/>
	<key>LSUIElement</key>
	<false/>
	<key>NSSupportsAutomaticGraphicsSwitching</key>
	<true/>
</dict>
</plist>`,
		ID,
		Name,
		Name,
		Version,
		Version,
		IconName[:len(IconName)-4],
	)

	plistPath := filepath.Join(contentsDir, "Info.plist")
	if err := os.WriteFile(plistPath, []byte(plistContent), 0o644); err != nil {
		return "", fmt.Errorf("write Info.plist: %w", err)
	}

	return appDir, nil
}
