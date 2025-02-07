// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package main

import (
	"errors"
	"fmt"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"

	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/opampsupervisor/supervisor"
)

var (
	kernel32API = windows.NewLazySystemDLL("kernel32.dll")

	allocConsoleProc = kernel32API.NewProc("AllocConsole")
	freeConsoleProc  = kernel32API.NewProc("FreeConsole")
)

func run() error {
	// always allocate a console in case we're running as service
	if err := allocConsole(); err != nil {
		if !errors.Is(err, windows.ERROR_ACCESS_DENIED) {
			// Per https://learn.microsoft.com/en-us/windows/console/allocconsole#remarks
			// AllocConsole fails with this error when there's already a console attached, such as not being ran as service
			// ignore this error and only return other errors
			return fmt.Errorf("alloc console: %w", err)
		}
	}
	defer func() {
		_ = freeConsole()
	}()

	// No need to supply service name when startup is invoked through
	// the Service Control Manager directly.
	if err := svc.Run("", supervisor.NewSvcHandler()); err != nil {
		if errors.Is(err, windows.ERROR_FAILED_SERVICE_CONTROLLER_CONNECT) {
			// Per https://learn.microsoft.com/en-us/windows/win32/api/winsvc/nf-winsvc-startservicectrldispatchera#return-value
			// this means that the process is not running as a service, so run interactively.

			return runInteractive()
		}
		return fmt.Errorf("failed to start supervisor: %w", err)
	}
	return nil
}

// windows services don't get created with a console
// need to allocate a console in order to send CTRL_BREAK_EVENT to agent sub process
func allocConsole() error {
	ret, _, err := allocConsoleProc.Call()
	if ret == 0 {
		return err
	}
	return nil
}

// free console once we're done with it
func freeConsole() error {
	ret, _, err := freeConsoleProc.Call()
	if ret == 0 {
		return err
	}
	return nil
}
