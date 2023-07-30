// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows
// +build windows

package subprocessmanager // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusexecreceiver/subprocessmanager"

import (
	"os"
	"os/exec"
	"syscall"
)

// Windows version of exec.Command(...)
// Compiles on Windows only
func ExecCommand(commandLine string) (*exec.Cmd, error) {

	var comSpec = os.Getenv("COMSPEC")
	if comSpec == "" {
		comSpec = os.Getenv("SystemRoot") + "\\System32\\cmd.exe"
	}
	childProcess := exec.Command(comSpec)                                                  // #nosec
	childProcess.SysProcAttr = &syscall.SysProcAttr{CmdLine: "/C \"" + commandLine + "\""} // #nosec

	return childProcess, nil

}
