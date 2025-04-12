// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ntpreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ntpreceiver"

import (
	"context"
	"time"

	"github.com/beevik/ntp"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ntpreceiver/internal/metadata"
)

type ntpScraper struct {
	logger   *zap.Logger
	mb       *metadata.MetricsBuilder
	version  int
	timeout  time.Duration
	endpoint string
}

func (s *ntpScraper) scrape(context.Context) (pmetric.Metrics, error) {
	options := ntp.QueryOptions{Version: s.version, Timeout: s.timeout}
	response, err := ntp.QueryWithOptions(s.endpoint, options)
	if err != nil {
		return pmetric.Metrics{}, err
	}
	s.mb.RecordNtpOffsetDataPoint(pcommon.NewTimestampFromTime(time.Now()), response.ClockOffset.Nanoseconds())
	s.mb.NewResourceBuilder().SetNtpHost(s.endpoint)
	return s.mb.Emit(), nil
}

func newScraper(cfg *Config, settings receiver.Settings) *ntpScraper {
	return &ntpScraper{
		logger:   settings.Logger,
		mb:       metadata.NewMetricsBuilder(cfg.MetricsBuilderConfig, settings),
		version:  cfg.Version,
		timeout:  cfg.Timeout,
		endpoint: cfg.Endpoint,
	}
}
