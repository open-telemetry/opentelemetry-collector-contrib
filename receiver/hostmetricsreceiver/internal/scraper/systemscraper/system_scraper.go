// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package systemscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/systemscraper"

import (
	"context"
	"time"

	"github.com/shirou/gopsutil/v4/common"
	"github.com/shirou/gopsutil/v4/host"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/systemscraper/internal/metadata"
)

// scraper for Uptime Metrics
type systemsScraper struct {
	settings receiver.Settings
	config   *Config
	mb       *metadata.MetricsBuilder

	// for mocking
	bootTime func(context.Context) (uint64, error)
	uptime   func(context.Context) (uint64, error)
}

// newUptimeScraper creates an Uptime related metric
func newUptimeScraper(_ context.Context, settings receiver.Settings, cfg *Config) *systemsScraper {
	return &systemsScraper{settings: settings, config: cfg, bootTime: host.BootTimeWithContext, uptime: host.UptimeWithContext}
}

func (s *systemsScraper) start(ctx context.Context, _ component.Host) error {
	ctx = context.WithValue(ctx, common.EnvKey, s.config.EnvMap)
	bootTime, err := s.bootTime(ctx)
	if err != nil {
		return err
	}

	s.mb = metadata.NewMetricsBuilder(s.config.MetricsBuilderConfig, s.settings, metadata.WithStartTime(pcommon.Timestamp(bootTime*1e9)))
	return nil
}

func (s *systemsScraper) scrape(ctx context.Context) (pmetric.Metrics, error) {
	now := pcommon.NewTimestampFromTime(time.Now())
	ctx = context.WithValue(ctx, common.EnvKey, s.config.EnvMap)

	uptime, err := s.uptime(ctx)
	if err != nil {
		return pmetric.NewMetrics(), err
	}

	s.mb.RecordSystemUptimeDataPoint(now, float64(uptime))

	return s.mb.Emit(), nil
}
