// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package commander

import (
	"os"
	"syscall"
)

func sendShutdownSignal(process *os.Process) error {
	return process.Signal(os.Interrupt)
}

func sysProcAttrs() *syscall.SysProcAttr {
	// On non-windows systems, no extra attributes are needed.
	return nil
}
