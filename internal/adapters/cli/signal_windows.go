//go:build windows

package cli

import "os"

func notifyExtraSignals(_ chan<- os.Signal) {
	// Windows 不支持 SIGQUIT，无需额外信号
}
