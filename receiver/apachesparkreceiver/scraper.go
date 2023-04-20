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

package apachesparkreceiver

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"
)

type sparkScraper struct {
	logger   *zap.Logger
	cfg      *Config
	settings component.TelemetrySettings
	// TODO: add when metadata is created
	// mb:       metadata.NewMetricsBuilder(cfg.MetricsBuilderConfig, settings),
}

func newScraper(logger *zap.Logger, cfg *Config, settings receiver.CreateSettings) *sparkScraper {
	return &sparkScraper{
		logger:   logger,
		cfg:      cfg,
		settings: settings.TelemetrySettings,
	}
}

func (s *sparkScraper) start(_ context.Context, _ component.Host) (err error) {
	return nil
}

func (s *sparkScraper) scrape(_ context.Context) (pmetric.Metrics, error) {
	md := pmetric.NewMetrics()

	return md, nil
}
