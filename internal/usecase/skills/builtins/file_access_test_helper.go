package builtins

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func writeFileAccessConfigForTest(t *testing.T, workspace string, enabled bool, allowedPaths []string) {
	t.Helper()

	configDir := filepath.Join(workspace, "config")
	require.NoError(t, os.MkdirAll(configDir, 0755))

	var b strings.Builder
	b.WriteString("server:\n")
	b.WriteString(fmt.Sprintf("  file_access:\n    enabled: %t\n", enabled))
	if len(allowedPaths) > 0 {
		b.WriteString("    allowed_paths:\n")
		for _, p := range allowedPaths {
			b.WriteString(fmt.Sprintf("      - %q\n", p))
		}
	}

	require.NoError(t, os.WriteFile(filepath.Join(configDir, "server.yml"), []byte(b.String()), 0644))
}
