// Copyright  The OpenTelemetry Authors
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

package flinkmetricsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/flinkmetricsreceiver"

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/flinkmetricsreceiver/internal/metadata"
)

var errClientNotInit = errors.New("client not initialized")

type flinkmetricsScraper struct {
	client   client
	cfg      *Config
	settings component.TelemetrySettings
	mb       *metadata.MetricsBuilder
}

func newflinkScraper(config *Config, settings component.TelemetrySettings) *flinkmetricsScraper {
	return &flinkmetricsScraper{
		settings: settings,
		cfg:      config,
		mb:       metadata.NewMetricsBuilder(config.Metrics),
	}
}

func (s *flinkmetricsScraper) start(_ context.Context, host component.Host) error {
	httpClient, err := newClient(s.cfg, host, s.settings, s.settings.Logger)
	if err != nil {
		return fmt.Errorf("failed to start: %w", err)
	}
	s.client = httpClient
	return nil
}

func (s *flinkmetricsScraper) scrape(ctx context.Context) (pmetric.Metrics, error) {
	// Validate we don't attempt to scrape without initializing the client
	if s.client == nil {
		return pmetric.NewMetrics(), errClientNotInit
	}

	now := pcommon.NewTimestampFromTime(time.Now())

	jobmanagerMetrics, err := s.client.GetJobmanagerMetrics(ctx)
	if err != nil {
		s.settings.Logger.Error("Failed to fetch jobmanager metrics",
			zap.String("endpoint", s.cfg.Endpoint),
			zap.Error(err),
		)
		return pmetric.NewMetrics(), err
	}

	taskmanagersMetrics, err := s.client.GetTaskmanagersMetrics(ctx)
	if err != nil {
		s.settings.Logger.Error("Failed to fetch taskmanager metrics",
			zap.String("endpoint", s.cfg.Endpoint),
			zap.Error(err),
		)
		return pmetric.NewMetrics(), err
	}

	jobsMetrics, err := s.client.GetJobsMetrics(ctx)
	if err != nil {
		s.settings.Logger.Error("Failed to fetch jobs metrics",
			zap.String("endpoint", s.cfg.Endpoint),
			zap.Error(err),
		)
		return pmetric.NewMetrics(), err
	}
	subtasksMetrics, err := s.client.GetSubtasksMetrics(ctx)
	if err != nil {
		s.settings.Logger.Error("Failed to fetch subtasks metrics",
			zap.String("endpoint", s.cfg.Endpoint),
			zap.Error(err),
		)
		return pmetric.NewMetrics(), err
	}

	s.processJobmanagerMetrics(now, jobmanagerMetrics)
	s.processTaskmanagerMetrics(now, taskmanagersMetrics)
	s.processJobsMetrics(now, jobsMetrics)
	s.processSubtaskMetrics(now, subtasksMetrics)

	return s.mb.Emit(), nil
}
