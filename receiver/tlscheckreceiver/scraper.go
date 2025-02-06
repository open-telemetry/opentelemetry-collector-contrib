// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tlscheckreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/tlscheckreceiver"

import (
	"context"

	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/tlscheckreceiver/internal/metadata"
)

type tlsCheckScraper struct {
	// include string
	logger *zap.Logger
	mb     *metadata.MetricsBuilder
}

func (s *tlsCheckScraper) scrape(_ context.Context) (pmetric.Metrics, error) {
	return pmetric.NewMetrics(), nil
}

func newScraper(cfg *Config, settings receiver.Settings) *tlsCheckScraper {
	return &tlsCheckScraper{
		logger: settings.TelemetrySettings.Logger,
		mb:     metadata.NewMetricsBuilder(cfg.MetricsBuilderConfig, settings),
	}
}
