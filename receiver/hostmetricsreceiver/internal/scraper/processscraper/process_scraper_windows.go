// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows
// +build windows

package processscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/processscraper"

import (
	"fmt"
	"path/filepath"
	"regexp"
)

func getProcessName(_ processHandle, exePath string) (string, error) {
	if exePath == "" {
		return "", fmt.Errorf("executable path is empty")
	}

	return filepath.Base(exePath), nil
}

func getProcessExecutable(proc processHandle) (string, error) {
	exe, err := proc.Exe()
	if err != nil {
		return "", err
	}

	return exe, nil
}

// matches the first argument before an unquoted space or slash
var cmdRegex = regexp.MustCompile(`^((?:[^"]*?"[^"]*?")*?[^"]*?)(?:[ \/]|$)`)

func getProcessCommand(proc processHandle) (*commandMetadata, error) {
	cmdline, err := proc.Cmdline()
	if err != nil {
		return nil, err
	}

	cmd := cmdline
	match := cmdRegex.FindStringSubmatch(cmdline)
	if match != nil {
		cmd = match[1]
	}

	command := &commandMetadata{command: cmd, commandLine: cmdline}
	return command, nil
}
