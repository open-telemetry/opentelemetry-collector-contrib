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
	"golang.org/x/exp/slices"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/apachesparkreceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/apachesparkreceiver/internal/models"
)

var (
	errClientNotInit             = errors.New("client not initialized")
	errFailedAppIDCollection     = errors.New("failed to retrieve app ids")
	errNoMatchingWhitelistedApps = errors.New("no apps matched whitelisted names")
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

	// call applications endpoint to get ids and names for all apps in the cluster
	var apps *models.Applications
	apps, err := s.client.GetApplications()
	if err != nil {
		return pmetric.NewMetrics(), errFailedAppIDCollection
	}

	// check apps against whitelisted app ids from config
	var whitelistedApps []models.Application

	// if no ids specified, whitelist all apps
	switch {
	case s.config.WhitelistedApplicationNames == nil:
		whitelistedApps = *apps
	case len(s.config.WhitelistedApplicationNames) == 0:
		whitelistedApps = *apps
		s.logger.Warn("Empty array of whitelisted application IDs specified - all applications will be monitored.")
	default:
		// some whitelisted ids specified, compare to ids from applications endpoint
		for _, app := range *apps {
			if slices.Contains(s.config.WhitelistedApplicationNames, app.Name) {
				whitelistedApps = append(whitelistedApps, app)
			}
		}
		if len(whitelistedApps) == 0 {
			return pmetric.NewMetrics(), errNoMatchingWhitelistedApps
		}
	}

	// get stats from the 'metrics' endpoint
	clusterStats, err := s.client.GetClusterStats()
	if err != nil {
		scrapeErrors.AddPartial(32, err)
		s.logger.Warn("Failed to scrape cluster stats", zap.Error(err))
	} else {
		for _, app := range whitelistedApps {
			s.collectCluster(clusterStats, now, app.ApplicationID, app.Name)
		}
	}

	// for each application id, get stats from stages & executors endpoints
	for _, app := range whitelistedApps {
		stageStats, err := s.client.GetStageStats(app.ApplicationID)
		if err != nil {
			scrapeErrors.AddPartial(24, err)
			s.logger.Warn("Failed to scrape stage stats", zap.Error(err))
		} else {
			s.collectStage(*stageStats, now, app.ApplicationID, app.Name)
		}

		executorStats, err := s.client.GetExecutorStats(app.ApplicationID)
		if err != nil {
			scrapeErrors.AddPartial(13, err)
			s.logger.Warn("Failed to scrape executor stats", zap.Error(err))
		} else {
			s.collectExecutor(*executorStats, now, app.ApplicationID, app.Name)
		}

		jobStats, err := s.client.GetJobStats(app.ApplicationID)
		if err != nil {
			scrapeErrors.AddPartial(8, err)
			s.logger.Warn("Failed to scrape job stats", zap.Error(err))
		} else {
			s.collectJob(*jobStats, now, app.ApplicationID, app.Name)
		}
	}
	return s.mb.Emit(), scrapeErrors.Combine()
}

func (s *sparkScraper) collectCluster(clusterStats *models.ClusterProperties, now pcommon.Timestamp, appID string, appName string) {
	if stat, ok := clusterStats.Gauges[fmt.Sprintf("%s.driver.BlockManager.disk.diskSpaceUsed_MB", appID)]; ok {
		s.mb.RecordSparkDriverBlockManagerDiskDiskSpaceUsedDataPoint(now, int64(stat.Value), appID, appName)
	}
	if stat, ok := clusterStats.Gauges[fmt.Sprintf("%s.driver.BlockManager.memory.offHeapMemUsed_MB", appID)]; ok {
		s.mb.RecordSparkDriverBlockManagerMemoryUsedDataPoint(now, int64(stat.Value), appID, appName, metadata.AttributeLocationOffHeap)
	}
	if stat, ok := clusterStats.Gauges[fmt.Sprintf("%s.driver.BlockManager.memory.onHeapMemUsed_MB", appID)]; ok {
		s.mb.RecordSparkDriverBlockManagerMemoryUsedDataPoint(now, int64(stat.Value), appID, appName, metadata.AttributeLocationOnHeap)
	}
	if stat, ok := clusterStats.Gauges[fmt.Sprintf("%s.driver.BlockManager.memory.remainingOffHeapMem_MB", appID)]; ok {
		s.mb.RecordSparkDriverBlockManagerMemoryRemainingDataPoint(now, int64(stat.Value), appID, appName, metadata.AttributeLocationOffHeap)
	}
	if stat, ok := clusterStats.Gauges[fmt.Sprintf("%s.driver.BlockManager.memory.remainingOnHeapMem_MB", appID)]; ok {
		s.mb.RecordSparkDriverBlockManagerMemoryRemainingDataPoint(now, int64(stat.Value), appID, appName, metadata.AttributeLocationOnHeap)
	}

	if stat, ok := clusterStats.Counters[fmt.Sprintf("%s.driver.HiveExternalCatalog.fileCacheHits", appID)]; ok {
		s.mb.RecordSparkDriverHiveExternalCatalogFileCacheHitsDataPoint(now, stat.Count, appID, appName)
	}
	if stat, ok := clusterStats.Counters[fmt.Sprintf("%s.driver.HiveExternalCatalog.filesDiscovered", appID)]; ok {
		s.mb.RecordSparkDriverHiveExternalCatalogFilesDiscoveredDataPoint(now, stat.Count, appID, appName)
	}
	if stat, ok := clusterStats.Counters[fmt.Sprintf("%s.driver.HiveExternalCatalog.hiveClientCalls", appID)]; ok {
		s.mb.RecordSparkDriverHiveExternalCatalogHiveClientCallsDataPoint(now, stat.Count, appID, appName)
	}
	if stat, ok := clusterStats.Counters[fmt.Sprintf("%s.driver.HiveExternalCatalog.parallelListingJobCount", appID)]; ok {
		s.mb.RecordSparkDriverHiveExternalCatalogParallelListingJobsDataPoint(now, stat.Count, appID, appName)
	}
	if stat, ok := clusterStats.Counters[fmt.Sprintf("%s.driver.HiveExternalCatalog.partitionsFetched", appID)]; ok {
		s.mb.RecordSparkDriverHiveExternalCatalogPartitionsFetchedDataPoint(now, stat.Count, appID, appName)
	}

	if stat, ok := clusterStats.Histograms[fmt.Sprintf("%s.driver.CodeGenerator.compilationTime", appID)]; ok {
		s.mb.RecordSparkDriverCodeGeneratorCompilationCountDataPoint(now, stat.Count, appID, appName)
		s.mb.RecordSparkDriverCodeGeneratorCompilationAverageTimeDataPoint(now, stat.Mean, appID, appName)
	}
	if stat, ok := clusterStats.Histograms[fmt.Sprintf("%s.driver.CodeGenerator.generatedClassSize", appID)]; ok {
		s.mb.RecordSparkDriverCodeGeneratorGeneratedClassCountDataPoint(now, stat.Count, appID, appName)
		s.mb.RecordSparkDriverCodeGeneratorGeneratedClassAverageSizeDataPoint(now, stat.Mean, appID, appName)
	}
	if stat, ok := clusterStats.Histograms[fmt.Sprintf("%s.driver.CodeGenerator.generatedMethodSize", appID)]; ok {
		s.mb.RecordSparkDriverCodeGeneratorGeneratedMethodCountDataPoint(now, stat.Count, appID, appName)
		s.mb.RecordSparkDriverCodeGeneratorGeneratedMethodAverageSizeDataPoint(now, stat.Mean, appID, appName)
	}
	if stat, ok := clusterStats.Histograms[fmt.Sprintf("%s.driver.CodeGenerator.sourceCodeSize", appID)]; ok {
		s.mb.RecordSparkDriverCodeGeneratorSourceCodeCountDataPoint(now, stat.Count, appID, appName)
		s.mb.RecordSparkDriverCodeGeneratorSourceCodeAverageSizeDataPoint(now, stat.Mean, appID, appName)
	}

	if stat, ok := clusterStats.Gauges[fmt.Sprintf("%s.driver.DAGScheduler.job.activeJobs", appID)]; ok {
		s.mb.RecordSparkDriverDagSchedulerJobActiveJobsDataPoint(now, int64(stat.Value), appID, appName)
	}
	if stat, ok := clusterStats.Gauges[fmt.Sprintf("%s.driver.DAGScheduler.job.allJobs", appID)]; ok {
		s.mb.RecordSparkDriverDagSchedulerJobAllJobsDataPoint(now, int64(stat.Value), appID, appName)
	}
	if stat, ok := clusterStats.Gauges[fmt.Sprintf("%s.driver.DAGScheduler.stage.failedStages", appID)]; ok {
		s.mb.RecordSparkDriverDagSchedulerStageFailedStagesDataPoint(now, int64(stat.Value), appID, appName)
	}
	if stat, ok := clusterStats.Gauges[fmt.Sprintf("%s.driver.DAGScheduler.stage.runningStages", appID)]; ok {
		s.mb.RecordSparkDriverDagSchedulerStageRunningStagesDataPoint(now, int64(stat.Value), appID, appName)
	}
	if stat, ok := clusterStats.Gauges[fmt.Sprintf("%s.driver.DAGScheduler.stage.waitingStages", appID)]; ok {
		s.mb.RecordSparkDriverDagSchedulerStageWaitingStagesDataPoint(now, int64(stat.Value), appID, appName)
	}

	if stat, ok := clusterStats.Counters[fmt.Sprintf("%s.driver.LiveListenerBus.numEventsPosted", appID)]; ok {
		s.mb.RecordSparkDriverLiveListenerBusEventsPostedDataPoint(now, stat.Count, appID, appName)
	}
	if stat, ok := clusterStats.Timers[fmt.Sprintf("%s.driver.LiveListenerBus.queue.appStatus.listenerProcessingTime", appID)]; ok {
		s.mb.RecordSparkDriverLiveListenerBusListenerProcessingTimeAverageDataPoint(now, stat.Mean, appID, appName)
	}
	if stat, ok := clusterStats.Counters[fmt.Sprintf("%s.driver.LiveListenerBus.queue.appStatus.numDroppedEvents", appID)]; ok {
		s.mb.RecordSparkDriverLiveListenerBusEventsDroppedDataPoint(now, stat.Count, appID, appName)
	}
	if stat, ok := clusterStats.Gauges[fmt.Sprintf("%s.driver.LiveListenerBus.queue.appStatus.size", appID)]; ok {
		s.mb.RecordSparkDriverLiveListenerBusQueueSizeDataPoint(now, int64(stat.Value), appID, appName)
	}

	if stat, ok := clusterStats.Gauges[fmt.Sprintf("%s.driver.JVMCPU.jvmCpuTime", appID)]; ok {
		s.mb.RecordSparkDriverJvmCPUTimeDataPoint(now, int64(stat.Value), appID, appName)
	}

	if stat, ok := clusterStats.Gauges[fmt.Sprintf("%s.driver.ExecutorMetrics.JVMOffHeapMemory", appID)]; ok {
		s.mb.RecordSparkDriverExecutorMetricsJvmMemoryDataPoint(now, int64(stat.Value), appID, appName, metadata.AttributeLocationOffHeap)
	}
	if stat, ok := clusterStats.Gauges[fmt.Sprintf("%s.driver.ExecutorMetrics.JVMHeapMemory", appID)]; ok {
		s.mb.RecordSparkDriverExecutorMetricsJvmMemoryDataPoint(now, int64(stat.Value), appID, appName, metadata.AttributeLocationOnHeap)
	}
	if stat, ok := clusterStats.Gauges[fmt.Sprintf("%s.driver.ExecutorMetrics.OffHeapExecutionMemory", appID)]; ok {
		s.mb.RecordSparkDriverExecutorMetricsExecutionMemoryDataPoint(now, int64(stat.Value), appID, appName, metadata.AttributeLocationOffHeap)
	}
	if stat, ok := clusterStats.Gauges[fmt.Sprintf("%s.driver.ExecutorMetrics.OnHeapExecutionMemory", appID)]; ok {
		s.mb.RecordSparkDriverExecutorMetricsExecutionMemoryDataPoint(now, int64(stat.Value), appID, appName, metadata.AttributeLocationOnHeap)
	}
	if stat, ok := clusterStats.Gauges[fmt.Sprintf("%s.driver.ExecutorMetrics.OffHeapStorageMemory", appID)]; ok {
		s.mb.RecordSparkDriverExecutorMetricsStorageMemoryDataPoint(now, int64(stat.Value), appID, appName, metadata.AttributeLocationOffHeap)
	}
	if stat, ok := clusterStats.Gauges[fmt.Sprintf("%s.driver.ExecutorMetrics.OnHeapStorageMemory", appID)]; ok {
		s.mb.RecordSparkDriverExecutorMetricsStorageMemoryDataPoint(now, int64(stat.Value), appID, appName, metadata.AttributeLocationOnHeap)
	}
	if stat, ok := clusterStats.Gauges[fmt.Sprintf("%s.driver.ExecutorMetrics.DirectPoolMemory", appID)]; ok {
		s.mb.RecordSparkDriverExecutorMetricsPoolMemoryDataPoint(now, int64(stat.Value), appID, appName, metadata.AttributePoolMemoryTypeDirect)
	}
	if stat, ok := clusterStats.Gauges[fmt.Sprintf("%s.driver.ExecutorMetrics.MappedPoolMemory", appID)]; ok {
		s.mb.RecordSparkDriverExecutorMetricsPoolMemoryDataPoint(now, int64(stat.Value), appID, appName, metadata.AttributePoolMemoryTypeMapped)
	}
	if stat, ok := clusterStats.Gauges[fmt.Sprintf("%s.driver.ExecutorMetrics.MinorGCCount", appID)]; ok {
		s.mb.RecordSparkDriverExecutorMetricsGcCountDataPoint(now, int64(stat.Value), appID, appName, metadata.AttributeGcTypeMinor)
	}
	if stat, ok := clusterStats.Gauges[fmt.Sprintf("%s.driver.ExecutorMetrics.MajorGCCount", appID)]; ok {
		s.mb.RecordSparkDriverExecutorMetricsGcCountDataPoint(now, int64(stat.Value), appID, appName, metadata.AttributeGcTypeMajor)
	}
	if stat, ok := clusterStats.Gauges[fmt.Sprintf("%s.driver.ExecutorMetrics.MinorGCTime", appID)]; ok {
		s.mb.RecordSparkDriverExecutorMetricsGcTimeDataPoint(now, int64(stat.Value), appID, appName, metadata.AttributeGcTypeMinor)
	}
	if stat, ok := clusterStats.Gauges[fmt.Sprintf("%s.driver.ExecutorMetrics.MajorGCTime", appID)]; ok {
		s.mb.RecordSparkDriverExecutorMetricsGcTimeDataPoint(now, int64(stat.Value), appID, appName, metadata.AttributeGcTypeMajor)
	}
}

func (s *sparkScraper) collectStage(stageStats models.Stages, now pcommon.Timestamp, appID string, appName string) {
	for i := range stageStats {
		var stageStatus metadata.AttributeStageStatus
		switch stageStats[i].Status {
		case "ACTIVE":
			stageStatus = metadata.AttributeStageStatusACTIVE
		case "COMPLETE":
			stageStatus = metadata.AttributeStageStatusCOMPLETE
		case "PENDING":
			stageStatus = metadata.AttributeStageStatusPENDING
		case "FAILED":
			stageStatus = metadata.AttributeStageStatusFAILED
		default:
			s.logger.Warn("Unsupported Spark stage status supplied: ignoring this stage's metrics and continuing to metrics for next stage", zap.String("status", stageStats[i].Status))
			continue
		}

		s.mb.RecordSparkStageActiveTasksDataPoint(now, stageStats[i].NumActiveTasks, appID, appName, stageStats[i].StageID, stageStats[i].AttemptID, stageStatus)
		s.mb.RecordSparkStageCompleteTasksDataPoint(now, stageStats[i].NumCompleteTasks, appID, appName, stageStats[i].StageID, stageStats[i].AttemptID, stageStatus)
		s.mb.RecordSparkStageFailedTasksDataPoint(now, stageStats[i].NumFailedTasks, appID, appName, stageStats[i].StageID, stageStats[i].AttemptID, stageStatus)
		s.mb.RecordSparkStageKilledTasksDataPoint(now, stageStats[i].NumKilledTasks, appID, appName, stageStats[i].StageID, stageStats[i].AttemptID, stageStatus)
		s.mb.RecordSparkStageExecutorRunTimeDataPoint(now, stageStats[i].ExecutorRunTime, appID, appName, stageStats[i].StageID, stageStats[i].AttemptID, stageStatus)
		s.mb.RecordSparkStageExecutorCPUTimeDataPoint(now, stageStats[i].ExecutorCPUTime, appID, appName, stageStats[i].StageID, stageStats[i].AttemptID, stageStatus)
		s.mb.RecordSparkStageResultSizeDataPoint(now, stageStats[i].ResultSize, appID, appName, stageStats[i].StageID, stageStats[i].AttemptID, stageStatus)
		s.mb.RecordSparkStageJvmGcTimeDataPoint(now, stageStats[i].JvmGcTime, appID, appName, stageStats[i].StageID, stageStats[i].AttemptID, stageStatus)
		s.mb.RecordSparkStageMemorySpilledDataPoint(now, stageStats[i].MemoryBytesSpilled, appID, appName, stageStats[i].StageID, stageStats[i].AttemptID, stageStatus)
		s.mb.RecordSparkStageDiskSpaceSpilledDataPoint(now, stageStats[i].DiskBytesSpilled, appID, appName, stageStats[i].StageID, stageStats[i].AttemptID, stageStatus)
		s.mb.RecordSparkStagePeakExecutionMemoryDataPoint(now, stageStats[i].PeakExecutionMemory, appID, appName, stageStats[i].StageID, stageStats[i].AttemptID, stageStatus)
		s.mb.RecordSparkStageInputBytesDataPoint(now, stageStats[i].InputBytes, appID, appName, stageStats[i].StageID, stageStats[i].AttemptID, stageStatus)
		s.mb.RecordSparkStageInputRecordsDataPoint(now, stageStats[i].InputRecords, appID, appName, stageStats[i].StageID, stageStats[i].AttemptID, stageStatus)
		s.mb.RecordSparkStageOutputBytesDataPoint(now, stageStats[i].OutputBytes, appID, appName, stageStats[i].StageID, stageStats[i].AttemptID, stageStatus)
		s.mb.RecordSparkStageOutputRecordsDataPoint(now, stageStats[i].OutputRecords, appID, appName, stageStats[i].StageID, stageStats[i].AttemptID, stageStatus)
		s.mb.RecordSparkStageShuffleBlocksFetchedDataPoint(now, stageStats[i].ShuffleRemoteBlocksFetched, appID, appName, stageStats[i].StageID, stageStats[i].AttemptID, stageStatus, metadata.AttributeSourceRemote)
		s.mb.RecordSparkStageShuffleBlocksFetchedDataPoint(now, stageStats[i].ShuffleLocalBlocksFetched, appID, appName, stageStats[i].StageID, stageStats[i].AttemptID, stageStatus, metadata.AttributeSourceLocal)
		s.mb.RecordSparkStageShuffleFetchWaitTimeDataPoint(now, stageStats[i].ShuffleFetchWaitTime, appID, appName, stageStats[i].StageID, stageStats[i].AttemptID, stageStatus)
		s.mb.RecordSparkStageShuffleBytesReadDataPoint(now, stageStats[i].ShuffleRemoteBytesRead, appID, appName, stageStats[i].StageID, stageStats[i].AttemptID, stageStatus, metadata.AttributeSourceRemote)
		s.mb.RecordSparkStageShuffleBytesReadDataPoint(now, stageStats[i].ShuffleLocalBytesRead, appID, appName, stageStats[i].StageID, stageStats[i].AttemptID, stageStatus, metadata.AttributeSourceLocal)
		s.mb.RecordSparkStageShuffleRemoteBytesReadToDiskDataPoint(now, stageStats[i].ShuffleRemoteBytesReadToDisk, appID, appName, stageStats[i].StageID, stageStats[i].AttemptID, stageStatus)
		s.mb.RecordSparkStageShuffleReadBytesDataPoint(now, stageStats[i].ShuffleReadBytes, appID, appName, stageStats[i].StageID, stageStats[i].AttemptID, stageStatus)
		s.mb.RecordSparkStageShuffleReadRecordsDataPoint(now, stageStats[i].ShuffleReadRecords, appID, appName, stageStats[i].StageID, stageStats[i].AttemptID, stageStatus)
		s.mb.RecordSparkStageShuffleWriteBytesDataPoint(now, stageStats[i].ShuffleWriteBytes, appID, appName, stageStats[i].StageID, stageStats[i].AttemptID, stageStatus)
		s.mb.RecordSparkStageShuffleWriteRecordsDataPoint(now, stageStats[i].ShuffleWriteRecords, appID, appName, stageStats[i].StageID, stageStats[i].AttemptID, stageStatus)
		s.mb.RecordSparkStageShuffleWriteTimeDataPoint(now, stageStats[i].ShuffleWriteTime, appID, appName, stageStats[i].StageID, stageStats[i].AttemptID, stageStatus)
	}
}

func (s *sparkScraper) collectExecutor(executorStats models.Executors, now pcommon.Timestamp, appID string, appName string) {
	for i := range executorStats {
		s.mb.RecordSparkExecutorMemoryUsedDataPoint(now, executorStats[i].MemoryUsed, appID, appName, executorStats[i].ExecutorID)
		s.mb.RecordSparkExecutorDiskUsedDataPoint(now, executorStats[i].DiskUsed, appID, appName, executorStats[i].ExecutorID)
		s.mb.RecordSparkExecutorMaxTasksDataPoint(now, executorStats[i].MaxTasks, appID, appName, executorStats[i].ExecutorID)
		s.mb.RecordSparkExecutorActiveTasksDataPoint(now, executorStats[i].ActiveTasks, appID, appName, executorStats[i].ExecutorID)
		s.mb.RecordSparkExecutorFailedTasksDataPoint(now, executorStats[i].FailedTasks, appID, appName, executorStats[i].ExecutorID)
		s.mb.RecordSparkExecutorCompletedTasksDataPoint(now, executorStats[i].CompletedTasks, appID, appName, executorStats[i].ExecutorID)
		s.mb.RecordSparkExecutorDurationDataPoint(now, executorStats[i].TotalDuration, appID, appName, executorStats[i].ExecutorID)
		s.mb.RecordSparkExecutorGcTimeDataPoint(now, executorStats[i].TotalGCTime, appID, appName, executorStats[i].ExecutorID)
		s.mb.RecordSparkExecutorInputBytesDataPoint(now, executorStats[i].TotalInputBytes, appID, appName, executorStats[i].ExecutorID)
		s.mb.RecordSparkExecutorShuffleReadBytesDataPoint(now, executorStats[i].TotalShuffleRead, appID, appName, executorStats[i].ExecutorID)
		s.mb.RecordSparkExecutorShuffleWriteBytesDataPoint(now, executorStats[i].TotalShuffleWrite, appID, appName, executorStats[i].ExecutorID)
		s.mb.RecordSparkExecutorUsedStorageMemoryDataPoint(now, executorStats[i].UsedOnHeapStorageMemory, appID, appName, executorStats[i].ExecutorID, metadata.AttributeLocationOnHeap)
		s.mb.RecordSparkExecutorUsedStorageMemoryDataPoint(now, executorStats[i].UsedOffHeapStorageMemory, appID, appName, executorStats[i].ExecutorID, metadata.AttributeLocationOffHeap)
		s.mb.RecordSparkExecutorTotalStorageMemoryDataPoint(now, executorStats[i].TotalOnHeapStorageMemory, appID, appName, executorStats[i].ExecutorID, metadata.AttributeLocationOnHeap)
		s.mb.RecordSparkExecutorTotalStorageMemoryDataPoint(now, executorStats[i].TotalOffHeapStorageMemory, appID, appName, executorStats[i].ExecutorID, metadata.AttributeLocationOffHeap)
	}
}

func (s *sparkScraper) collectJob(jobStats models.Jobs, now pcommon.Timestamp, appID string, appName string) {
	for i := range jobStats {
		s.mb.RecordSparkJobActiveTasksDataPoint(now, jobStats[i].NumActiveTasks, appID, appName, jobStats[i].JobID)
		s.mb.RecordSparkJobCompletedTasksDataPoint(now, jobStats[i].NumCompletedTasks, appID, appName, jobStats[i].JobID)
		s.mb.RecordSparkJobSkippedTasksDataPoint(now, jobStats[i].NumSkippedTasks, appID, appName, jobStats[i].JobID)
		s.mb.RecordSparkJobFailedTasksDataPoint(now, jobStats[i].NumFailedTasks, appID, appName, jobStats[i].JobID)
		s.mb.RecordSparkJobActiveStagesDataPoint(now, jobStats[i].NumActiveStages, appID, appName, jobStats[i].JobID)
		s.mb.RecordSparkJobCompletedStagesDataPoint(now, jobStats[i].NumCompletedStages, appID, appName, jobStats[i].JobID)
		s.mb.RecordSparkJobSkippedStagesDataPoint(now, jobStats[i].NumSkippedStages, appID, appName, jobStats[i].JobID)
		s.mb.RecordSparkJobFailedStagesDataPoint(now, jobStats[i].NumFailedStages, appID, appName, jobStats[i].JobID)
	}
}
