//go:build !windows

package cli

import (
	"os"
	"os/signal"
	"syscall"
)

func notifyExtraSignals(sigCh chan<- os.Signal) {
	signal.Notify(sigCh, syscall.SIGQUIT)
}
