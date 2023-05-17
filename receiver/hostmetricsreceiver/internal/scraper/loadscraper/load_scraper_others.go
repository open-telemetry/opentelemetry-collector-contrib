// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !windows
// +build !windows

package loadscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/loadscraper"

import (
	"context"

	"github.com/shirou/gopsutil/v3/load"
	"go.uber.org/zap"
)

// unix based systems sample & compute load averages in the kernel, so nothing to do here
func startSampling(_ context.Context, _ *zap.Logger) error {
	return nil
}

func stopSampling(_ context.Context) error {
	return nil
}

func getSampledLoadAverages() (*load.AvgStat, error) {
	return load.Avg()
}
