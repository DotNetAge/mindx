//go:build windows

package setup

import (
	"os/exec"
)

func setDetachAttrs(_ *exec.Cmd) {
	// On Windows, Process.Release() is sufficient to detach a child process.
	// No SysProcAttr equivalent of Setpgid is needed.
}
