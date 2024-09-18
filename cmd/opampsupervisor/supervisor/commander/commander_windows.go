// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package commander

import (
	"fmt"
	"os"
	"syscall"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
)

var (
	kernel32API = windows.NewLazySystemDLL("kernel32.dll")

	ctrlEventProc    = kernel32API.NewProc("GenerateConsoleCtrlEvent")
	allocConsoleProc = kernel32API.NewProc("AllocConsole")
	freeConsoleProc  = kernel32API.NewProc("FreeConsole")
)

func sendShutdownSignal(process *os.Process) error {
	// signaling with os.Interrupt is not supported on windows systems,
	// so we need to use the windows API to properly send a graceful shutdown signal.
	// See: https://learn.microsoft.com/en-us/windows/console/generateconsolectrlevent
	isService, err := svc.IsWindowsService()
	if err != nil {
		return fmt.Errorf("isWindowsService: %w", err)
	}

	if isService {
		if err := allocConsole(); err != nil {
			return fmt.Errorf("allocConsole: %w", err)
		}
	}

	r, _, e := ctrlEventProc.Call(syscall.CTRL_BREAK_EVENT, uintptr(process.Pid))
	if r == 0 {
		return fmt.Errorf("sendShutdownSignal to PID '%d': %w", process.Pid, e)
	}

	if isService {
		if err := freeConsole(); err != nil {
			return fmt.Errorf("freeConsole: %w", err)
		}
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

func allocConsole() error {
	// windows services don't get created with a console
	// need to allocate a console in order to send CTRL_BREAK_EVENT to agent sub process
	ret, _, err := allocConsoleProc.Call()
	if ret == 0 {
		return err
	}
	return nil
}

func freeConsole() error {
	// windows services don't run with console, so we want to clean it up
	// only keep console around while it is strictly necessary
	ret, _, err := freeConsoleProc.Call()
	if ret == 0 {
		return err
	}
	return nil
}
