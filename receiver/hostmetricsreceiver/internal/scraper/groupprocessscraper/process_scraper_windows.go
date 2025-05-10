// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package groupprocessscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/groupprocessscraper"

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
)

func getProcessName(_ context.Context, _ processHandle, exePath string) (string, error) {
	if exePath == "" {
		return "", fmt.Errorf("executable path is empty")
	}

	return filepath.Base(exePath), nil
}

func getProcessCgroup(_ context.Context, _ processHandle) (string, error) {
	return "", nil
}

func getProcessExecutable(ctx context.Context, proc processHandle) (string, error) {
	exe, err := proc.ExeWithContext(ctx)
	if err != nil {
		return "", err
	}

	return exe, nil
}

// matches the first argument before an unquoted space or slash
var cmdRegex = regexp.MustCompile(`^((?:[^"]*?"[^"]*?")*?[^"]*?)(?:[ \/]|$)`)

func getProcessCommand(ctx context.Context, proc processHandle) (*commandMetadata, error) {
	cmdline, err := proc.CmdlineWithContext(ctx)
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
