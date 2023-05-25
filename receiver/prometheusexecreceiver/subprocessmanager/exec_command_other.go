// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !windows
// +build !windows

package subprocessmanager // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusexecreceiver/subprocessmanager"

import (
	"os/exec"

	"github.com/kballard/go-shellquote"
)

// Non-Windows version of exec.Command(...)
// Compiles on all but Windows
func ExecCommand(commandLine string) (*exec.Cmd, error) {

	var args, err = shellquote.Split(commandLine)
	if err != nil {
		return nil, err
	}

	return exec.Command(args[0], args[1:]...), nil // #nosec

}
