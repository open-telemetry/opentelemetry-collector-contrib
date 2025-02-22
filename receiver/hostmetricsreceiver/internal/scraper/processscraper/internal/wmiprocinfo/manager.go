// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package wmiprocinfo // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/processscraper/internal/wmiprocinfo"

import "errors"

var ErrPlatformSupport = errors.New("WMI process info collection is only supported on Windows")

// Manager is the public interface for a WMI Process Info manager.
type Manager interface {
	Refresh() error
	GetProcessHandleCount(pid int64) (uint32, error)
	GetProcessPpid(pid int64) (int64, error)
}
