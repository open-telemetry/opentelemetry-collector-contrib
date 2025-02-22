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

type QueryOption func(string) string

// WithHandleCount includes the HandleCount field in the WMI query.
func WithHandleCount(q string) string {
	return q + ", HandleCount"
}

// WithParentProcessId includes the ParentProcessId field in the WMI query.
func WithParentProcessId(q string) string {
	return q + ", ParentProcessId"
}

func constructQueryString(queryOpts ...QueryOption) (string, error) {
	queryStr := "SELECT ProcessId"
	queryOptSet := false

	for _, opt := range queryOpts {
		queryStr = opt(queryStr)
		queryOptSet = true
	}

	if !queryOptSet {
		return "", errors.New("no query options supplied")
	}

	return queryStr + " FROM Win32_Process", nil
}
