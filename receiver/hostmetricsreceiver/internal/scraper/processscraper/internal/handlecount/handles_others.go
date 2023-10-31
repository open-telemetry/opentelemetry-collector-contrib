// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !windows
// +build !windows

package handlecount // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/processscraper/internal/handlecount"

import "errors"

var ErrHandlesPlatformSupport = errors.New("process handle collection is only supported on Windows")

func NewManager() Manager {
	return &unsupportedManager{}
}

type unsupportedManager struct{}

func (m *unsupportedManager) Refresh() error {
	return ErrHandlesPlatformSupport
}

func (m *unsupportedManager) GetProcessHandleCount(_ int64) (uint32, error) {
	return 0, ErrHandlesPlatformSupport
}
