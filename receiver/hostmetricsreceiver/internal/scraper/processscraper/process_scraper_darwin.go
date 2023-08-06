// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build darwin
// +build darwin

package processscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/processscraper"

import "regexp"

func getProcessName(proc processHandle, _ string) (string, error) {
	name, err := proc.Name()
	if err != nil {
		return "", err
	}

	return name, nil
}

func getProcessExecutable(proc processHandle) (string, error) {
	cmdline, err := proc.Cmdline()
	if err != nil {
		return "", err
	}
	regex := regexp.MustCompile(`^\S+`)
	exe := regex.FindString(cmdline)

	return exe, nil
}

func getProcessCommand(proc processHandle) (*commandMetadata, error) {
	cmdline, err := proc.Cmdline()
	if err != nil {
		return nil, err
	}

	command := &commandMetadata{command: cmdline, commandLine: cmdline}
	return command, nil

}
