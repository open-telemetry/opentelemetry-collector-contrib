// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package process

import (
	"os"
	"syscall"
)

// sysProcAttr puts the child in its own process group so that a console
// control event sent to the group does not reach the Supervisor.
func sysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP}
}

// terminate ends the child.
//
// Windows has no SIGTERM. A graceful stop needs a console control event sent
// to the process group, which the Supervisor does not do for the Collector
// either. A production implementation should send CTRL_BREAK_EVENT first.
func terminate(process *os.Process) error {
	return process.Kill()
}

// kill ends the child.
func kill(process *os.Process) error {
	return process.Kill()
}
