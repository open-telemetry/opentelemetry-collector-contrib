// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

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
		s.mb.RecordSparkDriverBlockManagerDiskUsageDataPoint(now, int64(stat.Value))
	}
	if stat, ok := clusterStats.Gauges[fmt.Sprintf("%s.driver.BlockManager.memory.offHeapMemUsed_MB", appID)]; ok {
		s.mb.RecordSparkDriverBlockManagerMemoryUsageDataPoint(now, int64(stat.Value), metadata.AttributeLocationOffHeap, metadata.AttributeStateUsed)
	}
	if stat, ok := clusterStats.Gauges[fmt.Sprintf("%s.driver.BlockManager.memory.onHeapMemUsed_MB", appID)]; ok {
		s.mb.RecordSparkDriverBlockManagerMemoryUsageDataPoint(now, int64(stat.Value), metadata.AttributeLocationOnHeap, metadata.AttributeStateUsed)
	}
	if stat, ok := clusterStats.Gauges[fmt.Sprintf("%s.driver.BlockManager.memory.remainingOffHeapMem_MB", appID)]; ok {
		s.mb.RecordSparkDriverBlockManagerMemoryUsageDataPoint(now, int64(stat.Value), metadata.AttributeLocationOffHeap, metadata.AttributeStateFree)
	}
	if stat, ok := clusterStats.Gauges[fmt.Sprintf("%s.driver.BlockManager.memory.remainingOnHeapMem_MB", appID)]; ok {
		s.mb.RecordSparkDriverBlockManagerMemoryUsageDataPoint(now, int64(stat.Value), metadata.AttributeLocationOnHeap, metadata.AttributeStateFree)
	}

	if stat, ok := clusterStats.Counters[fmt.Sprintf("%s.driver.HiveExternalCatalog.fileCacheHits", appID)]; ok {
		s.mb.RecordSparkDriverHiveExternalCatalogFileCacheHitsDataPoint(now, stat.Count)
	}
	if stat, ok := clusterStats.Counters[fmt.Sprintf("%s.driver.HiveExternalCatalog.filesDiscovered", appID)]; ok {
		s.mb.RecordSparkDriverHiveExternalCatalogFilesDiscoveredDataPoint(now, stat.Count)
	}
	if stat, ok := clusterStats.Counters[fmt.Sprintf("%s.driver.HiveExternalCatalog.hiveClientCalls", appID)]; ok {
		s.mb.RecordSparkDriverHiveExternalCatalogHiveClientCallsDataPoint(now, stat.Count)
	}
	if stat, ok := clusterStats.Counters[fmt.Sprintf("%s.driver.HiveExternalCatalog.parallelListingJobCount", appID)]; ok {
		s.mb.RecordSparkDriverHiveExternalCatalogParallelListingJobsDataPoint(now, stat.Count)
	}
	if stat, ok := clusterStats.Counters[fmt.Sprintf("%s.driver.HiveExternalCatalog.partitionsFetched", appID)]; ok {
		s.mb.RecordSparkDriverHiveExternalCatalogPartitionsFetchedDataPoint(now, stat.Count)
	}

	if stat, ok := clusterStats.Histograms[fmt.Sprintf("%s.driver.CodeGenerator.compilationTime", appID)]; ok {
		s.mb.RecordSparkDriverCodeGeneratorCompilationCountDataPoint(now, stat.Count)
		s.mb.RecordSparkDriverCodeGeneratorCompilationAverageTimeDataPoint(now, stat.Mean)
	}
	if stat, ok := clusterStats.Histograms[fmt.Sprintf("%s.driver.CodeGenerator.generatedClassSize", appID)]; ok {
		s.mb.RecordSparkDriverCodeGeneratorGeneratedClassCountDataPoint(now, stat.Count)
		s.mb.RecordSparkDriverCodeGeneratorGeneratedClassAverageSizeDataPoint(now, stat.Mean)
	}
	if stat, ok := clusterStats.Histograms[fmt.Sprintf("%s.driver.CodeGenerator.generatedMethodSize", appID)]; ok {
		s.mb.RecordSparkDriverCodeGeneratorGeneratedMethodCountDataPoint(now, stat.Count)
		s.mb.RecordSparkDriverCodeGeneratorGeneratedMethodAverageSizeDataPoint(now, stat.Mean)
	}
	if stat, ok := clusterStats.Histograms[fmt.Sprintf("%s.driver.CodeGenerator.sourceCodeSize", appID)]; ok {
		s.mb.RecordSparkDriverCodeGeneratorSourceCodeOperationsDataPoint(now, stat.Count)
		s.mb.RecordSparkDriverCodeGeneratorSourceCodeAverageSizeDataPoint(now, stat.Mean)
	}

	if stat, ok := clusterStats.Gauges[fmt.Sprintf("%s.driver.DAGScheduler.job.activeJobs", appID)]; ok {
		s.mb.RecordSparkDriverDagSchedulerJobActiveDataPoint(now, int64(stat.Value))
	}
	if stat, ok := clusterStats.Gauges[fmt.Sprintf("%s.driver.DAGScheduler.job.allJobs", appID)]; ok {
		s.mb.RecordSparkDriverDagSchedulerJobCountDataPoint(now, int64(stat.Value))
	}
	if stat, ok := clusterStats.Gauges[fmt.Sprintf("%s.driver.DAGScheduler.stage.failedStages", appID)]; ok {
		s.mb.RecordSparkDriverDagSchedulerStageFailedDataPoint(now, int64(stat.Value))
	}
	if stat, ok := clusterStats.Gauges[fmt.Sprintf("%s.driver.DAGScheduler.stage.runningStages", appID)]; ok {
		s.mb.RecordSparkDriverDagSchedulerStageCountDataPoint(now, int64(stat.Value), false, true)
	}
	if stat, ok := clusterStats.Gauges[fmt.Sprintf("%s.driver.DAGScheduler.stage.waitingStages", appID)]; ok {
		s.mb.RecordSparkDriverDagSchedulerStageCountDataPoint(now, int64(stat.Value), true, false)
	}

	if stat, ok := clusterStats.Counters[fmt.Sprintf("%s.driver.LiveListenerBus.numEventsPosted", appID)]; ok {
		s.mb.RecordSparkDriverLiveListenerBusPostedDataPoint(now, stat.Count)
	}
	if stat, ok := clusterStats.Timers[fmt.Sprintf("%s.driver.LiveListenerBus.queue.appStatus.listenerProcessingTime", appID)]; ok {
		s.mb.RecordSparkDriverLiveListenerBusProcessingTimeAverageDataPoint(now, stat.Mean)
	}
	if stat, ok := clusterStats.Counters[fmt.Sprintf("%s.driver.LiveListenerBus.queue.appStatus.numDroppedEvents", appID)]; ok {
		s.mb.RecordSparkDriverLiveListenerBusDroppedDataPoint(now, stat.Count)
	}
	if stat, ok := clusterStats.Gauges[fmt.Sprintf("%s.driver.LiveListenerBus.queue.appStatus.size", appID)]; ok {
		s.mb.RecordSparkDriverLiveListenerBusQueueSizeDataPoint(now, int64(stat.Value))
	}

	if stat, ok := clusterStats.Gauges[fmt.Sprintf("%s.driver.JVMCPU.jvmCpuTime", appID)]; ok {
		s.mb.RecordSparkDriverJvmCPUTimeDataPoint(now, int64(stat.Value))
	}

	if stat, ok := clusterStats.Gauges[fmt.Sprintf("%s.driver.ExecutorMetrics.JVMOffHeapMemory", appID)]; ok {
		s.mb.RecordSparkDriverExecutorMemoryJvmDataPoint(now, int64(stat.Value), metadata.AttributeLocationOffHeap)
	}
	if stat, ok := clusterStats.Gauges[fmt.Sprintf("%s.driver.ExecutorMetrics.JVMHeapMemory", appID)]; ok {
		s.mb.RecordSparkDriverExecutorMemoryJvmDataPoint(now, int64(stat.Value), metadata.AttributeLocationOnHeap)
	}
	if stat, ok := clusterStats.Gauges[fmt.Sprintf("%s.driver.ExecutorMetrics.OffHeapExecutionMemory", appID)]; ok {
		s.mb.RecordSparkDriverExecutorMemoryExecutionDataPoint(now, int64(stat.Value), metadata.AttributeLocationOffHeap)
	}
	if stat, ok := clusterStats.Gauges[fmt.Sprintf("%s.driver.ExecutorMetrics.OnHeapExecutionMemory", appID)]; ok {
		s.mb.RecordSparkDriverExecutorMemoryExecutionDataPoint(now, int64(stat.Value), metadata.AttributeLocationOnHeap)
	}
	if stat, ok := clusterStats.Gauges[fmt.Sprintf("%s.driver.ExecutorMetrics.OffHeapStorageMemory", appID)]; ok {
		s.mb.RecordSparkDriverExecutorMemoryStorageDataPoint(now, int64(stat.Value), metadata.AttributeLocationOffHeap)
	}
	if stat, ok := clusterStats.Gauges[fmt.Sprintf("%s.driver.ExecutorMetrics.OnHeapStorageMemory", appID)]; ok {
		s.mb.RecordSparkDriverExecutorMemoryStorageDataPoint(now, int64(stat.Value), metadata.AttributeLocationOnHeap)
	}
	if stat, ok := clusterStats.Gauges[fmt.Sprintf("%s.driver.ExecutorMetrics.DirectPoolMemory", appID)]; ok {
		s.mb.RecordSparkDriverExecutorMemoryPoolDataPoint(now, int64(stat.Value), metadata.AttributePoolMemoryTypeDirect)
	}
	if stat, ok := clusterStats.Gauges[fmt.Sprintf("%s.driver.ExecutorMetrics.MappedPoolMemory", appID)]; ok {
		s.mb.RecordSparkDriverExecutorMemoryPoolDataPoint(now, int64(stat.Value), metadata.AttributePoolMemoryTypeMapped)
	}
	if stat, ok := clusterStats.Gauges[fmt.Sprintf("%s.driver.ExecutorMetrics.MinorGCCount", appID)]; ok {
		s.mb.RecordSparkDriverExecutorGcOperationsDataPoint(now, int64(stat.Value), metadata.AttributeGcTypeMinor)
	}
	if stat, ok := clusterStats.Gauges[fmt.Sprintf("%s.driver.ExecutorMetrics.MajorGCCount", appID)]; ok {
		s.mb.RecordSparkDriverExecutorGcOperationsDataPoint(now, int64(stat.Value), metadata.AttributeGcTypeMajor)
	}
	if stat, ok := clusterStats.Gauges[fmt.Sprintf("%s.driver.ExecutorMetrics.MinorGCTime", appID)]; ok {
		s.mb.RecordSparkDriverExecutorGcTimeDataPoint(now, int64(stat.Value), metadata.AttributeGcTypeMinor)
	}
	if stat, ok := clusterStats.Gauges[fmt.Sprintf("%s.driver.ExecutorMetrics.MajorGCTime", appID)]; ok {
		s.mb.RecordSparkDriverExecutorGcTimeDataPoint(now, int64(stat.Value), metadata.AttributeGcTypeMajor)
	}

	s.mb.EmitForResource(metadata.WithSparkApplicationID(appID), metadata.WithSparkApplicationName(appName))
}

func (s *sparkScraper) collectStage(stageStats models.Stages, now pcommon.Timestamp, appID string, appName string) {
	for _, stat := range stageStats {
		switch stat.Status {
		case "ACTIVE":
			s.mb.RecordSparkStageStatusDataPoint(now, 0, true, false, false, false)
		case "COMPLETE":
			s.mb.RecordSparkStageStatusDataPoint(now, 0, false, true, false, false)
		case "PENDING":
			s.mb.RecordSparkStageStatusDataPoint(now, 0, false, false, true, false)
		case "FAILED":
			s.mb.RecordSparkStageStatusDataPoint(now, 0, false, false, false, true)
		default:
			s.logger.Warn("Unsupported Spark stage status supplied: ignoring this stage's metrics and continuing to metrics for next stage", zap.String("status", stat.Status))
			continue
		}

		s.mb.RecordSparkStageTaskActiveDataPoint(now, stat.NumActiveTasks)
		s.mb.RecordSparkStageTaskResultDataPoint(now, stat.NumCompleteTasks, metadata.AttributeStageTaskResultCompleted)
		s.mb.RecordSparkStageTaskResultDataPoint(now, stat.NumFailedTasks, metadata.AttributeStageTaskResultFailed)
		s.mb.RecordSparkStageTaskResultDataPoint(now, stat.NumKilledTasks, metadata.AttributeStageTaskResultKilled)
		s.mb.RecordSparkStageExecutorRunTimeDataPoint(now, stat.ExecutorRunTime)
		s.mb.RecordSparkStageExecutorCPUTimeDataPoint(now, stat.ExecutorCPUTime)
		s.mb.RecordSparkStageTaskResultSizeDataPoint(now, stat.ResultSize)
		s.mb.RecordSparkStageJvmGcTimeDataPoint(now, stat.JvmGcTime)
		s.mb.RecordSparkStageMemorySpilledDataPoint(now, stat.MemoryBytesSpilled)
		s.mb.RecordSparkStageDiskSpilledDataPoint(now, stat.DiskBytesSpilled)
		s.mb.RecordSparkStageMemoryPeakDataPoint(now, stat.PeakExecutionMemory)
		s.mb.RecordSparkStageIoSizeDataPoint(now, stat.InputBytes, metadata.AttributeDirectionIn)
		s.mb.RecordSparkStageIoSizeDataPoint(now, stat.OutputBytes, metadata.AttributeDirectionOut)
		s.mb.RecordSparkStageIoRecordsDataPoint(now, stat.InputRecords, metadata.AttributeDirectionIn)
		s.mb.RecordSparkStageIoRecordsDataPoint(now, stat.OutputRecords, metadata.AttributeDirectionOut)
		s.mb.RecordSparkStageShuffleBlocksFetchedDataPoint(now, stat.ShuffleRemoteBlocksFetched, metadata.AttributeSourceRemote)
		s.mb.RecordSparkStageShuffleBlocksFetchedDataPoint(now, stat.ShuffleLocalBlocksFetched, metadata.AttributeSourceLocal)
		s.mb.RecordSparkStageShuffleFetchWaitTimeDataPoint(now, stat.ShuffleFetchWaitTime)
		s.mb.RecordSparkStageShuffleIoDiskDataPoint(now, stat.ShuffleRemoteBytesReadToDisk)
		s.mb.RecordSparkStageShuffleIoReadSizeDataPoint(now, stat.ShuffleLocalBytesRead, metadata.AttributeSourceLocal)
		s.mb.RecordSparkStageShuffleIoReadSizeDataPoint(now, stat.ShuffleRemoteBytesRead, metadata.AttributeSourceRemote)
		s.mb.RecordSparkStageShuffleIoWriteSizeDataPoint(now, stat.ShuffleWriteBytes)
		s.mb.RecordSparkStageShuffleIoRecordsDataPoint(now, stat.ShuffleReadRecords, metadata.AttributeDirectionIn)
		s.mb.RecordSparkStageShuffleIoRecordsDataPoint(now, stat.ShuffleWriteRecords, metadata.AttributeDirectionOut)
		s.mb.RecordSparkStageShuffleWriteTimeDataPoint(now, stat.ShuffleWriteTime)

		s.mb.EmitForResource(metadata.WithSparkApplicationID(appID), metadata.WithSparkApplicationName(appName), metadata.WithSparkStageID(stat.StageID), metadata.WithSparkStageAttemptID(stat.AttemptID))
	}
}

func (s *sparkScraper) collectExecutor(executorStats models.Executors, now pcommon.Timestamp, appID string, appName string) {
	for _, stat := range executorStats {
		s.mb.RecordSparkExecutorMemoryUsageDataPoint(now, stat.MemoryUsed)
		s.mb.RecordSparkExecutorDiskUsageDataPoint(now, stat.DiskUsed)
		s.mb.RecordSparkExecutorTaskLimitDataPoint(now, stat.MaxTasks)
		s.mb.RecordSparkExecutorTaskActiveDataPoint(now, stat.ActiveTasks)
		s.mb.RecordSparkExecutorTaskResultDataPoint(now, stat.FailedTasks, metadata.AttributeExecutorTaskResultFailed)
		s.mb.RecordSparkExecutorTaskResultDataPoint(now, stat.CompletedTasks, metadata.AttributeExecutorTaskResultCompleted)
		s.mb.RecordSparkExecutorTimeDataPoint(now, stat.TotalDuration)
		s.mb.RecordSparkExecutorGcTimeDataPoint(now, stat.TotalGCTime)
		s.mb.RecordSparkExecutorInputSizeDataPoint(now, stat.TotalInputBytes)
		s.mb.RecordSparkExecutorShuffleIoSizeDataPoint(now, stat.TotalShuffleRead, metadata.AttributeDirectionIn)
		s.mb.RecordSparkExecutorShuffleIoSizeDataPoint(now, stat.TotalShuffleWrite, metadata.AttributeDirectionOut)
		used := stat.UsedOnHeapStorageMemory
		s.mb.RecordSparkExecutorStorageMemoryUsageDataPoint(now, used, metadata.AttributeLocationOnHeap, metadata.AttributeStateUsed)
		s.mb.RecordSparkExecutorStorageMemoryUsageDataPoint(now, stat.TotalOnHeapStorageMemory-used, metadata.AttributeLocationOnHeap, metadata.AttributeStateFree)
		used = stat.UsedOffHeapStorageMemory
		s.mb.RecordSparkExecutorStorageMemoryUsageDataPoint(now, used, metadata.AttributeLocationOffHeap, metadata.AttributeStateUsed)
		s.mb.RecordSparkExecutorStorageMemoryUsageDataPoint(now, stat.TotalOffHeapStorageMemory-used, metadata.AttributeLocationOffHeap, metadata.AttributeStateFree)

		s.mb.EmitForResource(metadata.WithSparkApplicationID(appID), metadata.WithSparkApplicationName(appName), metadata.WithSparkExecutorID(stat.ExecutorID))
	}
}

func (s *sparkScraper) collectJob(jobStats models.Jobs, now pcommon.Timestamp, appID string, appName string) {
	for _, stat := range jobStats {
		s.mb.RecordSparkJobTaskActiveDataPoint(now, stat.NumActiveTasks)
		s.mb.RecordSparkJobTaskResultDataPoint(now, stat.NumCompletedTasks, metadata.AttributeJobResultCompleted)
		s.mb.RecordSparkJobTaskResultDataPoint(now, stat.NumSkippedTasks, metadata.AttributeJobResultSkipped)
		s.mb.RecordSparkJobTaskResultDataPoint(now, stat.NumFailedTasks, metadata.AttributeJobResultFailed)
		s.mb.RecordSparkJobStageActiveDataPoint(now, stat.NumActiveStages)
		s.mb.RecordSparkJobStageResultDataPoint(now, stat.NumCompletedStages, metadata.AttributeJobResultCompleted)
		s.mb.RecordSparkJobStageResultDataPoint(now, stat.NumSkippedStages, metadata.AttributeJobResultSkipped)
		s.mb.RecordSparkJobStageResultDataPoint(now, stat.NumFailedStages, metadata.AttributeJobResultFailed)

		s.mb.EmitForResource(metadata.WithSparkApplicationID(appID), metadata.WithSparkApplicationName(appName), metadata.WithSparkJobID(stat.JobID))
	}
}
