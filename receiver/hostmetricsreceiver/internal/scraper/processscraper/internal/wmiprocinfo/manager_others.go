// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package wmiprocinfo // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/processscraper/internal/wmiprocinfo"

func NewManager() Manager {
	return &unsupportedManager{}
}

type unsupportedManager struct{}

func (m *unsupportedManager) Refresh() error {
	return ErrPlatformSupport
}

func (m *unsupportedManager) GetProcessHandleCount(_ int64) (uint32, error) {
	return 0, ErrPlatformSupport
}

func (m *unsupportedManager) GetProcessPpid(pid int64) (int64, error) {
	return 0, ErrPlatformSupport
}
