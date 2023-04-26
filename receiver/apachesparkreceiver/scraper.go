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
	"fmt"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/scrapererror"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/apachesparkreceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/apachesparkreceiver/internal/models"
)

var (
	errClientNotInit = errors.New("client not initialized")
)

type sparkScraper struct {
	client   client
	logger   *zap.Logger
	config   *Config
	settings component.TelemetrySettings
	mb       *metadata.MetricsBuilder
}

func newSparkScraper(logger *zap.Logger, cfg *Config, settings receiver.CreateSettings) *sparkScraper {
	return &sparkScraper{
		logger:   logger,
		config:   cfg,
		settings: settings.TelemetrySettings,
		mb:       metadata.NewMetricsBuilder(cfg.MetricsBuilderConfig, settings),
	}
}

func (s *sparkScraper) start(_ context.Context, host component.Host) (err error) {
	httpClient, err := newApacheSparkClient(s.config, host, s.settings)
	if err != nil {
		return fmt.Errorf("failed to start: %w", err)
	}
	s.client = httpClient
	return nil
}

func (s *sparkScraper) scrape(_ context.Context) (pmetric.Metrics, error) {
	now := pcommon.NewTimestampFromTime(time.Now())
	var scrapeErrors scrapererror.ScrapeErrors

	if s.client == nil {
		return pmetric.NewMetrics(), errClientNotInit
	}

	// call applications endpoint
	// not getting app name for now, just ids
	appIds, err := s.client.GetApplicationIDs()
	if err != nil {
		scrapeErrors.AddPartial(1, err)
		s.logger.Warn("Failed to scrape application ids", zap.Error(err))
	}

	// get stats from the 'metrics' endpoint
	clusterStats, err := s.client.GetClusterStats()
	if err != nil {
		scrapeErrors.AddPartial(1, err)
		s.logger.Warn("Failed to scrape cluster stats", zap.Error(err))
	}

	for _, appID := range appIds {
		s.collectCluster(clusterStats, now, appID)
	}

	// for each application id, get stats from stages & executors endpoints
	for _, appID := range appIds {

		stageStats, err := s.client.GetStageStats(appID)
		if err != nil {
			scrapeErrors.AddPartial(1, err)
			s.logger.Warn("Failed to scrape stage stats", zap.Error(err))
		}
		s.collectStage(*stageStats, now, appID)

		executorStats, err := s.client.GetExecutorStats(appID)
		if err != nil {
			scrapeErrors.AddPartial(1, err)
			s.logger.Warn("Failed to scrape executor stats", zap.Error(err))
		}
		s.collectExecutor(*executorStats, now, appID)

		jobStats, err := s.client.GetJobStats(appID)
		if err != nil {
			scrapeErrors.AddPartial(1, err)
			s.logger.Warn("Failed to scrape job stats", zap.Error(err))
		}
		s.collectJob(*jobStats, now, appID)
	}

	return s.mb.Emit(), scrapeErrors.Combine()
}

func (s *sparkScraper) collectCluster(clusterStats *models.ClusterProperties, now pcommon.Timestamp, appID string) {
	key := fmt.Sprintf("%s.driver.BlockManager.memory.offHeapMemUsed_MB", appID)
	s.mb.RecordSparkDriverBlockManagerMemoryUsedDataPoint(now, int64(clusterStats.Gauges[key].Value), appID, metadata.AttributeLocationOffHeap)
	s.mb.RecordSparkDriverBlockManagerMemoryUsedDataPoint(now, int64(clusterStats.Gauges[key].Value), appID, metadata.AttributeLocationOnHeap)
}

func (s *sparkScraper) collectStage(stageStats models.Stages, now pcommon.Timestamp, appID string) {
	s.mb.RecordSparkStageExecutorRunTimeDataPoint(now, int64(stageStats[0].ExecutorRunTime), appID)
}

func (s *sparkScraper) collectExecutor(executorStats models.Executors, now pcommon.Timestamp, appID string) {
	s.mb.RecordSparkExecutorMemoryUsedDataPoint(now, int64(executorStats[0].MemoryUsed), appID)
}

func (s *sparkScraper) collectJob(jobStats models.Jobs, now pcommon.Timestamp, appID string) {
	s.mb.RecordSparkJobActiveTasksDataPoint(now, int64(jobStats[0].NumActiveTasks), appID)
}
