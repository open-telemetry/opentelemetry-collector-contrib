// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package commander

import (
	"fmt"
	"os"
	"syscall"

	"golang.org/x/sys/windows"
)

var (
	kernel32API = windows.NewLazySystemDLL("kernel32.dll")

	ctrlEventProc = kernel32API.NewProc("GenerateConsoleCtrlEvent")
)

func sendShutdownSignal(process *os.Process) error {
	// signaling with os.Interrupt is not supported on windows systems,
	// so we need to use the windows API to properly send a graceful shutdown signal.
	// See: https://learn.microsoft.com/en-us/windows/console/generateconsolectrlevent
	r, _, e := ctrlEventProc.Call(syscall.CTRL_BREAK_EVENT, uintptr(process.Pid))
	if r == 0 {
		return fmt.Errorf("sendShutdownSignal to PID '%d': %w", process.Pid, e)
	}

	return nil
}

func sysProcAttrs() *syscall.SysProcAttr {
	// By default, a ctrl-break event applies to a whole process group, which ends up
	// shutting down the supervisor. Instead, we spawn the collector in its own process
	// group, so that sending a ctrl-break event shuts down just the collector.
	return &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}
}

// openAgentLogFile opens the file that captures the managed agent's
// stdout/stderr. Unlike Unix, Go's os.O_APPEND does not survive handle
// inheritance on Windows: the agent child inherits the raw handle and writes at
// its own file offset, so after an external copytruncate-style rotation
// truncates the file, the child's next write recreates the old size as a
// zero-filled hole. Opening with a pure FILE_APPEND_DATA handle (no
// FILE_WRITE_DATA) instead makes the kernel force every write through the
// handle - including the inherited child's - to the current end-of-file, so
// rotation actually reclaims space.
func openAgentLogFile(path string) (*os.File, error) {
	// A FILE_APPEND_DATA-only handle cannot truncate on open, so truncate first
	// through a throwaway read/write handle to match O_TRUNC on Unix.
	trunc, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return nil, err
	}
	trunc.Close()

	pathp, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	handle, err := windows.CreateFile(
		pathp,
		windows.FILE_APPEND_DATA,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_ATTRIBUTE_NORMAL,
		0,
	)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(handle), path), nil
}
