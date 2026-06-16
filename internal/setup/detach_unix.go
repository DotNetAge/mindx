//go:build !windows

package setup

import (
	"os/exec"
	"syscall"
)

func setDetachAttrs(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}
