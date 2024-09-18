// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package main

import (
	"errors"
	"fmt"

	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/opampsupervisor/supervisor"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
)

func run() error {
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
