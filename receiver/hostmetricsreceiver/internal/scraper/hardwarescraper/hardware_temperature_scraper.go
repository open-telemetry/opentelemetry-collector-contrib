// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !linux

package hardwarescraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/hardwarescraper"

import (
	"context"

	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/hardwarescraper/internal/metadata"
)

// hardwareTemperatureScraper scrapes temperature metrics from hardware sensors
type hardwareTemperatureScraper struct {
	logger               *zap.Logger
	config               *TemperatureConfig
	hwmonPath            string
	metricsBuilderConfig metadata.MetricsBuilderConfig
}

func (s *hardwareTemperatureScraper) start(_ context.Context) error {
	s.logger.Info("Temperature scraping is not supported on this platform")
	return ErrHwmonUnavailable
}

func (*hardwareTemperatureScraper) scrape(_ context.Context, _ *metadata.MetricsBuilder) error {
	return ErrHwmonUnavailable
}
