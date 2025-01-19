// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package loadscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/loadscraper"

import (
	"context"
	"time"

	"github.com/shirou/gopsutil/v4/load"
	"go.uber.org/zap"
)

// no sampling performed on non windows environments, nothing to do
func setSamplingFrequency(_ time.Duration) {}

// unix based systems sample & compute load averages in the kernel, so nothing to do here
func startSampling(_ context.Context, _ *zap.Logger) error {
	return nil
}

func stopSampling(_ context.Context) error {
	return nil
}

func getSampledLoadAverages(ctx context.Context) (*load.AvgStat, error) {
	return load.AvgWithContext(ctx)
}
