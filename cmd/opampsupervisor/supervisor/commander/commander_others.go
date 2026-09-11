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

// openAgentLogFile opens the file that captures the managed agent's
// stdout/stderr. O_APPEND makes each write land at the current end-of-file; on
// Unix this flag lives on the kernel open file description, which the agent
// child process inherits across exec, so its writes append to EOF too - which
// is what lets an external copytruncate-style rotation reclaim space instead of
// the file snapping back on the next write.
func openAgentLogFile(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC|os.O_APPEND, 0o644)
}
