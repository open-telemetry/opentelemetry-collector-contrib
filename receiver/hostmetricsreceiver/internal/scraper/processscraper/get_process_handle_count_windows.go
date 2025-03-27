// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package processscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/processscraper"

import (
	"context"
)

const handleCountMetricsLen = 1

func (p *wrappedProcessHandle) GetProcessHandleCountWithContext(ctx context.Context) (int64, error) {
	// On Windows NumFDsWithContext returns the number of open handles, since it uses the
	// GetProcessHandleCount API.
	fds, err := p.Process.NumFDsWithContext(ctx)
	return int64(fds), err
}
