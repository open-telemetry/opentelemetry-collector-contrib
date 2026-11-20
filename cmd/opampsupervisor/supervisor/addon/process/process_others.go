// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package process

import (
	"os"
	"syscall"
)

// sysProcAttr puts the child in its own process group so that signaling it
// also reaches the processes it starts.
func sysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setpgid: true}
}

// terminate asks the child's process group to exit. SIGTERM rather than
// SIGINT: SIGTERM is what service managers send, and a shell wrapper ignores
// SIGINT while waiting on its own child.
func terminate(process *os.Process) error {
	return signalGroup(process, syscall.SIGTERM)
}

// kill kills the child's process group.
func kill(process *os.Process) error {
	return signalGroup(process, syscall.SIGKILL)
}

// signalGroup sends sig to the process group led by process, falling back to
// the process alone if it does not lead a group.
func signalGroup(process *os.Process, sig syscall.Signal) error {
	if err := syscall.Kill(-process.Pid, sig); err != nil {
		if err == syscall.ESRCH {
			return os.ErrProcessDone
		}
		return process.Signal(sig)
	}
	return nil
}
