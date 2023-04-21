// Copyright 2020 OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package apachesparkreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/apachesparkreceiver"

import (
	"context"
	"errors"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/scrapererror"
	"go.uber.org/zap"
)

var (
	errClientNotInit    = errors.New("client not initialized")
	errScrapedNoMetrics = errors.New("failed to scrape any metrics")
)

type sparkScraper struct {
	client   client // match client type from Ian's code
	logger   *zap.Logger
	cfg      *Config
	settings component.TelemetrySettings
	// TODO: add back after generated metric files are created in internal/metadata dir
	// mb:       metadata.NewMetricsBuilder(cfg.MetricsBuilderConfig, settings),
}

func newScraper(logger *zap.Logger, cfg *Config, settings receiver.CreateSettings) *sparkScraper {
	return &sparkScraper{
		logger:   logger,
		cfg:      cfg,
		settings: settings.TelemetrySettings,
	}
}

func (s *sparkScraper) start(_ context.Context, host component.Host) (err error) {
	s.client, err := newApacheSparkClient(s.cfg, host, s.settings, s.logger)
	if err != nil {
		return err
	}
	return nil
}

func (s *sparkScraper) scrape(_ context.Context) (pmetric.Metrics, error) {
	now := pcommon.NewTimestampFromTime(time.Now())
	var scrapeErrors scrapererror.ScrapeErrors

	if s.client == nil {
		return pmetric.NewMetrics(), errClientNotInit
	}

	// get stats from the 'metrics' endpoint
	clusterStats, err := s.client.GetStats("/metrics/json")

	// call applications endpoint
	applicationStats, err := s.client.GetStats("/applications")
	if err != nil {
		scrapeErrors.AddPartial(1, err)
		s.logger.Warn("Failed to scrape application stats", zap.Error(err))
	}
	// determine application ids

	// for each application id, get stats from stages & executors endpoints
	stageStats, err := s.client.GetStats("/applications/APP_ID_HERE/stages")
	if err != nil {
		scrapeErrors.AddPartial(1, err)
		s.logger.Warn("Failed to scrape stage stats", zap.Error(err))
	}

	executorStats, err := s.client.GetStats("/applications/APP_ID_HERE/executors")
	if err != nil {
		scrapeErrors.AddPartial(1, err)
		s.logger.Warn("Failed to scrape executor stats", zap.Error(err))
	}
	md := pmetric.NewMetrics()

	return md, nil
}
