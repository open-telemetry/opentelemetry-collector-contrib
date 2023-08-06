// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !linux && !windows && !darwin
// +build !linux,!windows,!darwin

package processscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/processscraper"

func getProcessName(processHandle, string) (string, error) {
	return "", nil
}

func getProcessExecutable(processHandle) (string, error) {
	return "", nil
}

func getProcessCommand(processHandle) (*commandMetadata, error) {
	return nil, nil
}
