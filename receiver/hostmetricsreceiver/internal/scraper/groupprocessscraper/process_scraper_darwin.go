// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build darwin

package groupprocessscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/groupprocessscraper"

import (
	"context"
	"regexp"
)

func getProcessName(ctx context.Context, proc processHandle, _ string) (string, error) {
	name, err := proc.NameWithContext(ctx)
	if err != nil {
		return "", err
	}

	return name, nil
}

func getProcessCgroup(_ context.Context, _ processHandle) (string, error) {
	return "", nil
}

func getProcessExecutable(ctx context.Context, proc processHandle) (string, error) {
	cmdline, err := proc.CmdlineWithContext(ctx)
	if err != nil {
		return "", err
	}
	regex := regexp.MustCompile(`^\S+`)
	exe := regex.FindString(cmdline)

	return exe, nil
}

func getProcessCommand(ctx context.Context, proc processHandle) (*commandMetadata, error) {
	cmdline, err := proc.CmdlineWithContext(ctx)
	if err != nil {
		return nil, err
	}

	command := &commandMetadata{command: cmdline, commandLine: cmdline}
	return command, nil

}
