// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build linux
// +build linux

package subprocess // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/jmxreceiver/internal/subprocess"

import (
	"os/exec"
	"syscall"
)

func applyOSSpecificCmdModifications(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		// This is Linux-specific and will cause the subprocess to be killed by the OS if
		// the collector dies
		Pdeathsig: syscall.SIGTERM,
	}
}
