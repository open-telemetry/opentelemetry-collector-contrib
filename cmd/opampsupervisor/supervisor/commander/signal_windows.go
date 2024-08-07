//go:build windows

package commander

import (
	"os"
	"syscall"

	"golang.org/x/sys/windows"
)

var (
	win32API = windows.NewLazySystemDLL("kernel32.dll")

	ctrlEventProc = win32API.NewProc("GenerateConsoleCtrlEvent")
)

func sendShutdownSignal(process *os.Process) error {
	// signalling with os.Interrupt is not supported on windows systems,
	// so we need to use the windows API to properly send a graceful shutdown signal.
	// See: https://learn.microsoft.com/en-us/windows/console/generateconsolectrlevent
	r, _, e := ctrlEventProc.Call(syscall.CTRL_BREAK_EVENT, uintptr(process.Pid))
	if r == 0 {
		return e
	}

	return nil
}
