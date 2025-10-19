// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package icmpcheckreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/icmpcheckreceiver"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/icmpcheckreceiver/internal/metadata"
)

const (
	DefaultPingCount    = 3
	DefaultPingTimeout  = time.Second * 5
	DefaultPingInterval = time.Second * 1
)

type pingResult struct {
	stats      *pingStats
	targetHost string
	targetIP   string
	err        error
}

type icmpCheckScraper struct {
	cfg           *Config
	settings      component.TelemetrySettings
	mb            *metadata.MetricsBuilder
	pingerFactory func(target PingTarget) (pinger, error)
}

// apply defaults on targets if not set
func (scr *icmpCheckScraper) start(_ context.Context, _ component.Host) (err error) {
	if scr.pingerFactory == nil {
		scr.pingerFactory = defaultPingerFactory
	}
	for i := range scr.cfg.Targets {
		if scr.cfg.Targets[i].PingCount == 0 {
			scr.cfg.Targets[i].PingCount = DefaultPingCount
		}
		if scr.cfg.Targets[i].PingTimeout == 0 {
			scr.cfg.Targets[i].PingTimeout = DefaultPingTimeout
		}
		if scr.cfg.Targets[i].PingInterval == 0 {
			scr.cfg.Targets[i].PingInterval = DefaultPingInterval
		}
	}
	return nil
}

func (scr *icmpCheckScraper) scrape(_ context.Context) (pmetric.Metrics, error) {
	results := make(chan pingResult, len(scr.cfg.Targets))

	for _, target := range scr.cfg.Targets {
		p, err := scr.pingerFactory(target)
		if err != nil {
			results <- pingResult{
				targetHost: target.Host,
				err:        err,
			}
			continue
		}
		go ping(p, results)
	}

	for range scr.cfg.Targets {
		result := <-results
		addMetrics(result, scr.mb, scr.settings.Logger)
	}

	metrics := scr.mb.Emit()
	return metrics, nil
}

func ping(p pinger, results chan<- pingResult) {
	err := p.Run()
	if err != nil {
		results <- pingResult{
			targetHost: p.HostName(),
			err:        err,
		}
		return
	}

	results <- pingResult{
		targetHost: p.HostName(),
		targetIP:   p.IPString(),
		stats:      p.Stats(),
		err:        nil,
	}
}

func addMetrics(result pingResult, mb *metadata.MetricsBuilder, logger *zap.Logger) {
	now := pcommon.NewTimestampFromTime(time.Now())

	if result.err != nil {
		logger.Error(
			"failed to ping host",
			zap.String("host", result.targetHost),
			zap.Error(result.err))
		return
	}

	// Record all metrics - they will be filtered by MetricsBuilderConfig
	mb.RecordPingRttMinDataPoint(
		now,
		result.stats.minRtt.Milliseconds(),
		result.targetHost,
		result.targetIP)

	mb.RecordPingRttMaxDataPoint(
		now,
		result.stats.maxRtt.Milliseconds(),
		result.targetHost,
		result.targetIP)

	mb.RecordPingRttAvgDataPoint(
		now,
		result.stats.avgRtt.Milliseconds(),
		result.targetHost,
		result.targetIP)

	mb.RecordPingLossRatioDataPoint(
		now,
		result.stats.lossRato,
		result.targetHost,
		result.targetIP)
}

func newScraper(cfg *Config, settings receiver.Settings) *icmpCheckScraper {
	return &icmpCheckScraper{
		cfg:      cfg,
		settings: settings.TelemetrySettings,
		mb:       metadata.NewMetricsBuilder(cfg.MetricsBuilderConfig, settings),
	}
}
