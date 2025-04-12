// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package processscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/processscraper"

import (
	"context"
	"errors"
)

const handleCountMetricsLen = 0

var ErrHandlesPlatformSupport = errors.New("process handle collection is only supported on Windows")

func (p *wrappedProcessHandle) GetProcessHandleCountWithContext(_ context.Context) (int64, error) {
	return 0, ErrHandlesPlatformSupport
}
