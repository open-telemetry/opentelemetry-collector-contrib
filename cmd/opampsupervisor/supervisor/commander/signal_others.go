//go:build !windows

package commander

import "os"

func sendShutdownSignal(process *os.Process) error {
	return process.Signal(os.Interrupt)
}
