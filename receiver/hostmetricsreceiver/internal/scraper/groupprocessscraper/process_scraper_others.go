// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !linux && !windows && !darwin

package groupprocessscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/groupprocessscraper"

import (
	"context"
)

func getProcessName(context.Context, processHandle, string) (string, error) {
	return "", nil
}

func getProcessCgroup(ctx context.Context, proc processHandle) (string, error) {
	return "", nil
}

func getProcessExecutable(context.Context, processHandle) (string, error) {
	return "", nil
}

func getProcessCommand(context.Context, processHandle) (*commandMetadata, error) {
	return nil, nil
}
