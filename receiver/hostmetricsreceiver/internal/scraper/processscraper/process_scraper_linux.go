// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build linux
// +build linux

package processscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/processscraper"

func getProcessName(proc processHandle, _ string) (string, error) {
	name, err := proc.Name()
	if err != nil {
		return "", err
	}

	return name, err
}

func getProcessExecutable(proc processHandle) (string, error) {
	exe, err := proc.Exe()
	if err != nil {
		return "", err
	}

	return exe, nil
}

func getProcessCommand(proc processHandle) (*commandMetadata, error) {
	cmdline, err := proc.CmdlineSlice()
	if err != nil {
		return nil, err
	}

	var cmd string
	if len(cmdline) > 0 {
		cmd = cmdline[0]
	}

	command := &commandMetadata{command: cmd, commandLineSlice: cmdline}
	return command, nil
}
