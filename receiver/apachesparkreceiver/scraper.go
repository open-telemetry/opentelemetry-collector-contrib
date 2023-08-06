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

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/apachesparkreceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/apachesparkreceiver/internal/models"
)

var (
	errFailedAppIDCollection = errors.New("failed to retrieve app ids")
	errNoMatchingAllowedApps = errors.New("no apps matched allowed names")
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

	// Call applications endpoint to get ids and names for all apps in the cluster
	apps, err := s.client.Applications()
	if err != nil {
		return pmetric.NewMetrics(), errFailedAppIDCollection
	}

	// Check apps against allowed app names from config
	var allowedApps []models.Application

	// If no app names specified, allow all apps
	switch {
	case len(s.config.ApplicationNames) == 0:
		allowedApps = apps
	default:
		// Some allowed app names specified, compare to app names from applications endpoint
		appMap := make(map[string][]models.Application)
		for _, app := range apps {
			appMap[app.Name] = append(appMap[app.Name], app)
		}

		for _, name := range s.config.ApplicationNames {
			if apps, ok := appMap[name]; ok {
				allowedApps = append(allowedApps, apps...)
			}
		}
		if len(allowedApps) == 0 {
			return pmetric.NewMetrics(), errNoMatchingAllowedApps
		}
	}

	// Get stats from the 'metrics' endpoint
	clusterStats, err := s.client.ClusterStats()
	if err != nil {
		scrapeErrors.AddPartial(32, err)
		s.logger.Warn("Failed to scrape cluster stats", zap.Error(err))
	} else {
		for _, app := range allowedApps {
			s.recordCluster(clusterStats, now, app.ApplicationID, app.Name)
		}
	}

	// For each application id, get stats from stages & executors endpoints
	for _, app := range allowedApps {
		stageStats, err := s.client.StageStats(app.ApplicationID)
		if err != nil {
			scrapeErrors.AddPartial(24, err)
			s.logger.Warn("Failed to scrape stage stats", zap.Error(err))
		} else {
			s.recordStages(stageStats, now, app.ApplicationID, app.Name)
		}

		executorStats, err := s.client.ExecutorStats(app.ApplicationID)
		if err != nil {
			scrapeErrors.AddPartial(13, err)
			s.logger.Warn("Failed to scrape executor stats", zap.Error(err))
		} else {
			s.recordExecutors(executorStats, now, app.ApplicationID, app.Name)
		}

		jobStats, err := s.client.JobStats(app.ApplicationID)
		if err != nil {
			scrapeErrors.AddPartial(8, err)
			s.logger.Warn("Failed to scrape job stats", zap.Error(err))
		} else {
			s.recordJobs(jobStats, now, app.ApplicationID, app.Name)
		}
	}
	return s.mb.Emit(), scrapeErrors.Combine()
}

func (s *sparkScraper) recordCluster(clusterStats *models.ClusterProperties, now pcommon.Timestamp, appID string, appName string) {
	rb := s.mb.NewResourceBuilder()
	rb.SetSparkApplicationID(appID)
	rb.SetSparkApplicationName(appName)
	rmb := s.mb.ResourceMetricsBuilder(rb.Emit())

	if stat, ok := clusterStats.Gauges[fmt.Sprintf("%s.driver.BlockManager.disk.diskSpaceUsed_MB", appID)]; ok {
		rmb.RecordSparkDriverBlockManagerDiskUsageDataPoint(now, int64(stat.Value))
	}
	if stat, ok := clusterStats.Gauges[fmt.Sprintf("%s.driver.BlockManager.memory.offHeapMemUsed_MB", appID)]; ok {
		rmb.RecordSparkDriverBlockManagerMemoryUsageDataPoint(now, int64(stat.Value),
			metadata.AttributeLocationOffHeap, metadata.AttributeStateUsed)
	}
	if stat, ok := clusterStats.Gauges[fmt.Sprintf("%s.driver.BlockManager.memory.onHeapMemUsed_MB", appID)]; ok {
		rmb.RecordSparkDriverBlockManagerMemoryUsageDataPoint(now, int64(stat.Value),
			metadata.AttributeLocationOnHeap, metadata.AttributeStateUsed)
	}
	if stat, ok := clusterStats.Gauges[fmt.Sprintf("%s.driver.BlockManager.memory.remainingOffHeapMem_MB", appID)]; ok {
		rmb.RecordSparkDriverBlockManagerMemoryUsageDataPoint(now, int64(stat.Value),
			metadata.AttributeLocationOffHeap, metadata.AttributeStateFree)
	}
	if stat, ok := clusterStats.Gauges[fmt.Sprintf("%s.driver.BlockManager.memory.remainingOnHeapMem_MB", appID)]; ok {
		rmb.RecordSparkDriverBlockManagerMemoryUsageDataPoint(now, int64(stat.Value),
			metadata.AttributeLocationOnHeap, metadata.AttributeStateFree)
	}

	if stat, ok := clusterStats.Counters[fmt.Sprintf("%s.driver.HiveExternalCatalog.fileCacheHits", appID)]; ok {
		rmb.RecordSparkDriverHiveExternalCatalogFileCacheHitsDataPoint(now, stat.Count)
	}
	if stat, ok := clusterStats.Counters[fmt.Sprintf("%s.driver.HiveExternalCatalog.filesDiscovered", appID)]; ok {
		rmb.RecordSparkDriverHiveExternalCatalogFilesDiscoveredDataPoint(now, stat.Count)
	}
	if stat, ok := clusterStats.Counters[fmt.Sprintf("%s.driver.HiveExternalCatalog.hiveClientCalls", appID)]; ok {
		rmb.RecordSparkDriverHiveExternalCatalogHiveClientCallsDataPoint(now, stat.Count)
	}
	if stat, ok := clusterStats.Counters[fmt.Sprintf("%s.driver.HiveExternalCatalog.parallelListingJobCount", appID)]; ok {
		rmb.RecordSparkDriverHiveExternalCatalogParallelListingJobsDataPoint(now, stat.Count)
	}
	if stat, ok := clusterStats.Counters[fmt.Sprintf("%s.driver.HiveExternalCatalog.partitionsFetched", appID)]; ok {
		rmb.RecordSparkDriverHiveExternalCatalogPartitionsFetchedDataPoint(now, stat.Count)
	}

	if stat, ok := clusterStats.Histograms[fmt.Sprintf("%s.driver.CodeGenerator.compilationTime", appID)]; ok {
		rmb.RecordSparkDriverCodeGeneratorCompilationCountDataPoint(now, stat.Count)
		rmb.RecordSparkDriverCodeGeneratorCompilationAverageTimeDataPoint(now, stat.Mean)
	}
	if stat, ok := clusterStats.Histograms[fmt.Sprintf("%s.driver.CodeGenerator.generatedClassSize", appID)]; ok {
		rmb.RecordSparkDriverCodeGeneratorGeneratedClassCountDataPoint(now, stat.Count)
		rmb.RecordSparkDriverCodeGeneratorGeneratedClassAverageSizeDataPoint(now, stat.Mean)
	}
	if stat, ok := clusterStats.Histograms[fmt.Sprintf("%s.driver.CodeGenerator.generatedMethodSize", appID)]; ok {
		rmb.RecordSparkDriverCodeGeneratorGeneratedMethodCountDataPoint(now, stat.Count)
		rmb.RecordSparkDriverCodeGeneratorGeneratedMethodAverageSizeDataPoint(now, stat.Mean)
	}
	if stat, ok := clusterStats.Histograms[fmt.Sprintf("%s.driver.CodeGenerator.sourceCodeSize", appID)]; ok {
		rmb.RecordSparkDriverCodeGeneratorSourceCodeOperationsDataPoint(now, stat.Count)
		rmb.RecordSparkDriverCodeGeneratorSourceCodeAverageSizeDataPoint(now, stat.Mean)
	}

	if stat, ok := clusterStats.Gauges[fmt.Sprintf("%s.driver.DAGScheduler.job.activeJobs", appID)]; ok {
		rmb.RecordSparkDriverDagSchedulerJobActiveDataPoint(now, int64(stat.Value))
	}
	if stat, ok := clusterStats.Gauges[fmt.Sprintf("%s.driver.DAGScheduler.job.allJobs", appID)]; ok {
		rmb.RecordSparkDriverDagSchedulerJobCountDataPoint(now, int64(stat.Value))
	}
	if stat, ok := clusterStats.Gauges[fmt.Sprintf("%s.driver.DAGScheduler.stage.failedStages", appID)]; ok {
		rmb.RecordSparkDriverDagSchedulerStageFailedDataPoint(now, int64(stat.Value))
	}
	if stat, ok := clusterStats.Gauges[fmt.Sprintf("%s.driver.DAGScheduler.stage.runningStages", appID)]; ok {
		rmb.RecordSparkDriverDagSchedulerStageCountDataPoint(now, int64(stat.Value),
			metadata.AttributeSchedulerStatusRunning)
	}
	if stat, ok := clusterStats.Gauges[fmt.Sprintf("%s.driver.DAGScheduler.stage.waitingStages", appID)]; ok {
		rmb.RecordSparkDriverDagSchedulerStageCountDataPoint(now, int64(stat.Value),
			metadata.AttributeSchedulerStatusWaiting)
	}

	if stat, ok := clusterStats.Counters[fmt.Sprintf("%s.driver.LiveListenerBus.numEventsPosted", appID)]; ok {
		rmb.RecordSparkDriverLiveListenerBusPostedDataPoint(now, stat.Count)
	}
	if stat, ok := clusterStats.Timers[fmt.Sprintf("%s.driver.LiveListenerBus.queue.appStatus.listenerProcessingTime", appID)]; ok {
		rmb.RecordSparkDriverLiveListenerBusProcessingTimeAverageDataPoint(now, stat.Mean)
	}
	if stat, ok := clusterStats.Counters[fmt.Sprintf("%s.driver.LiveListenerBus.queue.appStatus.numDroppedEvents", appID)]; ok {
		rmb.RecordSparkDriverLiveListenerBusDroppedDataPoint(now, stat.Count)
	}
	if stat, ok := clusterStats.Gauges[fmt.Sprintf("%s.driver.LiveListenerBus.queue.appStatus.size", appID)]; ok {
		rmb.RecordSparkDriverLiveListenerBusQueueSizeDataPoint(now, int64(stat.Value))
	}

	if stat, ok := clusterStats.Gauges[fmt.Sprintf("%s.driver.JVMCPU.jvmCpuTime", appID)]; ok {
		rmb.RecordSparkDriverJvmCPUTimeDataPoint(now, int64(stat.Value))
	}

	if stat, ok := clusterStats.Gauges[fmt.Sprintf("%s.driver.ExecutorMetrics.JVMOffHeapMemory", appID)]; ok {
		rmb.RecordSparkDriverExecutorMemoryJvmDataPoint(now, int64(stat.Value), metadata.AttributeLocationOffHeap)
	}
	if stat, ok := clusterStats.Gauges[fmt.Sprintf("%s.driver.ExecutorMetrics.JVMHeapMemory", appID)]; ok {
		rmb.RecordSparkDriverExecutorMemoryJvmDataPoint(now, int64(stat.Value), metadata.AttributeLocationOnHeap)
	}
	if stat, ok := clusterStats.Gauges[fmt.Sprintf("%s.driver.ExecutorMetrics.OffHeapExecutionMemory", appID)]; ok {
		rmb.RecordSparkDriverExecutorMemoryExecutionDataPoint(now, int64(stat.Value), metadata.AttributeLocationOffHeap)
	}
	if stat, ok := clusterStats.Gauges[fmt.Sprintf("%s.driver.ExecutorMetrics.OnHeapExecutionMemory", appID)]; ok {
		rmb.RecordSparkDriverExecutorMemoryExecutionDataPoint(now, int64(stat.Value), metadata.AttributeLocationOnHeap)
	}
	if stat, ok := clusterStats.Gauges[fmt.Sprintf("%s.driver.ExecutorMetrics.OffHeapStorageMemory", appID)]; ok {
		rmb.RecordSparkDriverExecutorMemoryStorageDataPoint(now, int64(stat.Value), metadata.AttributeLocationOffHeap)
	}
	if stat, ok := clusterStats.Gauges[fmt.Sprintf("%s.driver.ExecutorMetrics.OnHeapStorageMemory", appID)]; ok {
		rmb.RecordSparkDriverExecutorMemoryStorageDataPoint(now, int64(stat.Value), metadata.AttributeLocationOnHeap)
	}
	if stat, ok := clusterStats.Gauges[fmt.Sprintf("%s.driver.ExecutorMetrics.DirectPoolMemory", appID)]; ok {
		rmb.RecordSparkDriverExecutorMemoryPoolDataPoint(now, int64(stat.Value), metadata.AttributePoolMemoryTypeDirect)
	}
	if stat, ok := clusterStats.Gauges[fmt.Sprintf("%s.driver.ExecutorMetrics.MappedPoolMemory", appID)]; ok {
		rmb.RecordSparkDriverExecutorMemoryPoolDataPoint(now, int64(stat.Value), metadata.AttributePoolMemoryTypeMapped)
	}
	if stat, ok := clusterStats.Gauges[fmt.Sprintf("%s.driver.ExecutorMetrics.MinorGCCount", appID)]; ok {
		rmb.RecordSparkDriverExecutorGcOperationsDataPoint(now, int64(stat.Value), metadata.AttributeGcTypeMinor)
	}
	if stat, ok := clusterStats.Gauges[fmt.Sprintf("%s.driver.ExecutorMetrics.MajorGCCount", appID)]; ok {
		rmb.RecordSparkDriverExecutorGcOperationsDataPoint(now, int64(stat.Value), metadata.AttributeGcTypeMajor)
	}
	if stat, ok := clusterStats.Gauges[fmt.Sprintf("%s.driver.ExecutorMetrics.MinorGCTime", appID)]; ok {
		rmb.RecordSparkDriverExecutorGcTimeDataPoint(now, int64(stat.Value), metadata.AttributeGcTypeMinor)
	}
	if stat, ok := clusterStats.Gauges[fmt.Sprintf("%s.driver.ExecutorMetrics.MajorGCTime", appID)]; ok {
		rmb.RecordSparkDriverExecutorGcTimeDataPoint(now, int64(stat.Value), metadata.AttributeGcTypeMajor)
	}
}

func (s *sparkScraper) recordStages(stageStats []models.Stage, now pcommon.Timestamp, appID string, appName string) {
	for _, stage := range stageStats {
		rb := s.mb.NewResourceBuilder()
		rb.SetSparkApplicationID(appID)
		rb.SetSparkApplicationName(appName)
		rb.SetSparkStageID(stage.StageID)
		rb.SetSparkStageAttemptID(stage.AttemptID)
		rmb := s.mb.ResourceMetricsBuilder(rb.Emit())

		switch stage.Status {
		case "ACTIVE":
			rmb.RecordSparkStageStatusDataPoint(now, 0, true, false, false, false)
		case "COMPLETE":
			rmb.RecordSparkStageStatusDataPoint(now, 0, false, true, false, false)
		case "PENDING":
			rmb.RecordSparkStageStatusDataPoint(now, 0, false, false, true, false)
		case "FAILED":
			rmb.RecordSparkStageStatusDataPoint(now, 0, false, false, false, true)
		default:
			s.logger.Warn("Unsupported Spark stage status supplied: ignoring this stage's metrics and continuing to metrics for next stage", zap.String("status", stage.Status))
			continue
		}

		rmb.RecordSparkStageTaskActiveDataPoint(now, stage.NumActiveTasks)
		rmb.RecordSparkStageTaskResultDataPoint(now, stage.NumCompleteTasks, metadata.AttributeStageTaskResultCompleted)
		rmb.RecordSparkStageTaskResultDataPoint(now, stage.NumFailedTasks, metadata.AttributeStageTaskResultFailed)
		rmb.RecordSparkStageTaskResultDataPoint(now, stage.NumKilledTasks, metadata.AttributeStageTaskResultKilled)
		rmb.RecordSparkStageExecutorRunTimeDataPoint(now, stage.ExecutorRunTime)
		rmb.RecordSparkStageExecutorCPUTimeDataPoint(now, stage.ExecutorCPUTime)
		rmb.RecordSparkStageTaskResultSizeDataPoint(now, stage.ResultSize)
		rmb.RecordSparkStageJvmGcTimeDataPoint(now, stage.JvmGcTime)
		rmb.RecordSparkStageMemorySpilledDataPoint(now, stage.MemoryBytesSpilled)
		rmb.RecordSparkStageDiskSpilledDataPoint(now, stage.DiskBytesSpilled)
		rmb.RecordSparkStageMemoryPeakDataPoint(now, stage.PeakExecutionMemory)
		rmb.RecordSparkStageIoSizeDataPoint(now, stage.InputBytes, metadata.AttributeDirectionIn)
		rmb.RecordSparkStageIoSizeDataPoint(now, stage.OutputBytes, metadata.AttributeDirectionOut)
		rmb.RecordSparkStageIoRecordsDataPoint(now, stage.InputRecords, metadata.AttributeDirectionIn)
		rmb.RecordSparkStageIoRecordsDataPoint(now, stage.OutputRecords, metadata.AttributeDirectionOut)
		rmb.RecordSparkStageShuffleBlocksFetchedDataPoint(now, stage.ShuffleRemoteBlocksFetched,
			metadata.AttributeSourceRemote)
		rmb.RecordSparkStageShuffleBlocksFetchedDataPoint(now, stage.ShuffleLocalBlocksFetched,
			metadata.AttributeSourceLocal)
		rmb.RecordSparkStageShuffleFetchWaitTimeDataPoint(now, stage.ShuffleFetchWaitTime)
		rmb.RecordSparkStageShuffleIoDiskDataPoint(now, stage.ShuffleRemoteBytesReadToDisk)
		rmb.RecordSparkStageShuffleIoReadSizeDataPoint(now, stage.ShuffleLocalBytesRead, metadata.AttributeSourceLocal)
		rmb.RecordSparkStageShuffleIoReadSizeDataPoint(now, stage.ShuffleRemoteBytesRead,
			metadata.AttributeSourceRemote)
		rmb.RecordSparkStageShuffleIoWriteSizeDataPoint(now, stage.ShuffleWriteBytes)
		rmb.RecordSparkStageShuffleIoRecordsDataPoint(now, stage.ShuffleReadRecords, metadata.AttributeDirectionIn)
		rmb.RecordSparkStageShuffleIoRecordsDataPoint(now, stage.ShuffleWriteRecords, metadata.AttributeDirectionOut)
		rmb.RecordSparkStageShuffleWriteTimeDataPoint(now, stage.ShuffleWriteTime)
	}
}

func (s *sparkScraper) recordExecutors(executorStats []models.Executor, now pcommon.Timestamp, appID string, appName string) {
	for _, executor := range executorStats {
		rb := s.mb.NewResourceBuilder()
		rb.SetSparkApplicationID(appID)
		rb.SetSparkApplicationName(appName)
		rb.SetSparkExecutorID(executor.ExecutorID)
		rmb := s.mb.ResourceMetricsBuilder(rb.Emit())

		rmb.RecordSparkExecutorMemoryUsageDataPoint(now, executor.MemoryUsed)
		rmb.RecordSparkExecutorDiskUsageDataPoint(now, executor.DiskUsed)
		rmb.RecordSparkExecutorTaskLimitDataPoint(now, executor.MaxTasks)
		rmb.RecordSparkExecutorTaskActiveDataPoint(now, executor.ActiveTasks)
		rmb.RecordSparkExecutorTaskResultDataPoint(now, executor.FailedTasks,
			metadata.AttributeExecutorTaskResultFailed)
		rmb.RecordSparkExecutorTaskResultDataPoint(now, executor.CompletedTasks,
			metadata.AttributeExecutorTaskResultCompleted)
		rmb.RecordSparkExecutorTimeDataPoint(now, executor.TotalDuration)
		rmb.RecordSparkExecutorGcTimeDataPoint(now, executor.TotalGCTime)
		rmb.RecordSparkExecutorInputSizeDataPoint(now, executor.TotalInputBytes)
		rmb.RecordSparkExecutorShuffleIoSizeDataPoint(now, executor.TotalShuffleRead, metadata.AttributeDirectionIn)
		rmb.RecordSparkExecutorShuffleIoSizeDataPoint(now, executor.TotalShuffleWrite, metadata.AttributeDirectionOut)
		used := executor.UsedOnHeapStorageMemory
		rmb.RecordSparkExecutorStorageMemoryUsageDataPoint(now, used, metadata.AttributeLocationOnHeap,
			metadata.AttributeStateUsed)
		rmb.RecordSparkExecutorStorageMemoryUsageDataPoint(now, executor.TotalOnHeapStorageMemory-used,
			metadata.AttributeLocationOnHeap, metadata.AttributeStateFree)
		used = executor.UsedOffHeapStorageMemory
		rmb.RecordSparkExecutorStorageMemoryUsageDataPoint(now, used, metadata.AttributeLocationOffHeap,
			metadata.AttributeStateUsed)
		rmb.RecordSparkExecutorStorageMemoryUsageDataPoint(now, executor.TotalOffHeapStorageMemory-used,
			metadata.AttributeLocationOffHeap, metadata.AttributeStateFree)
	}
}

func (s *sparkScraper) recordJobs(jobStats []models.Job, now pcommon.Timestamp, appID string, appName string) {
	for _, job := range jobStats {
		rb := s.mb.NewResourceBuilder()
		rb.SetSparkApplicationID(appID)
		rb.SetSparkApplicationName(appName)
		rb.SetSparkJobID(job.JobID)
		rmb := s.mb.ResourceMetricsBuilder(rb.Emit())

		rmb.RecordSparkJobTaskActiveDataPoint(now, job.NumActiveTasks)
		rmb.RecordSparkJobTaskResultDataPoint(now, job.NumCompletedTasks, metadata.AttributeJobResultCompleted)
		rmb.RecordSparkJobTaskResultDataPoint(now, job.NumSkippedTasks, metadata.AttributeJobResultSkipped)
		rmb.RecordSparkJobTaskResultDataPoint(now, job.NumFailedTasks, metadata.AttributeJobResultFailed)
		rmb.RecordSparkJobStageActiveDataPoint(now, job.NumActiveStages)
		rmb.RecordSparkJobStageResultDataPoint(now, job.NumCompletedStages, metadata.AttributeJobResultCompleted)
		rmb.RecordSparkJobStageResultDataPoint(now, job.NumSkippedStages, metadata.AttributeJobResultSkipped)
		rmb.RecordSparkJobStageResultDataPoint(now, job.NumFailedStages, metadata.AttributeJobResultFailed)
	}
}
