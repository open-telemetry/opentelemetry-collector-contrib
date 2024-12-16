// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package groupprocessscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/groupprocessscraper"

import (
	"context"
)

func getProcessName(ctx context.Context, proc processHandle, _ string) (string, error) {
	name, err := proc.NameWithContext(ctx)
	if err != nil {
		return "", err
	}

	return name, err
}

func getProcessExecutable(ctx context.Context, proc processHandle) (string, error) {
	exe, err := proc.ExeWithContext(ctx)
	if err != nil {
		return "", err
	}

	return exe, nil
}

func getProcessCgroup(ctx context.Context, proc processHandle) (string, error) {
	cgroup, err := proc.CgroupWithContext(ctx)
	if err != nil {
		return "", err
	}

	return cgroup, nil
}

func getProcessCommand(ctx context.Context, proc processHandle) (*commandMetadata, error) {
	cmdline, err := proc.CmdlineSliceWithContext(ctx)
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
