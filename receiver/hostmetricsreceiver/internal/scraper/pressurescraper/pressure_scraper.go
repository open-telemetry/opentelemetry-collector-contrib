// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pressurescraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/pressurescraper"

import (
	"context"
	"time"

	"github.com/shirou/gopsutil/v4/host"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/scraper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/pressurescraper/internal/metadata"
)

type pressureScraper struct {
	settings scraper.Settings
	config   *Config
	mb       *metadata.MetricsBuilder

	bootTime func(context.Context) (uint64, error)
}

func newPressureScraper(_ context.Context, settings scraper.Settings, cfg *Config) *pressureScraper {
	return &pressureScraper{
		settings: settings,
		config:   cfg,
		bootTime: host.BootTimeWithContext,
	}
}

func (s *pressureScraper) start(ctx context.Context, _ component.Host) error {
	bootTime, err := s.bootTime(ctx)
	if err != nil {
		return err
	}

	s.mb = metadata.NewMetricsBuilder(s.config.MetricsBuilderConfig, s.settings, metadata.WithStartTime(pcommon.Timestamp(bootTime*1e9)))
	return nil
}

func (s *pressureScraper) scrape(_ context.Context) (pmetric.Metrics, error) {
	now := pcommon.NewTimestampFromTime(time.Now())
	if err := s.recordPressureMetrics(now); err != nil {
		return pmetric.NewMetrics(), err
	}
	return s.mb.Emit(), nil
}
