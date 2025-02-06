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
	"go.opentelemetry.io/collector/scraper/scrapererror"
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

func newSparkScraper(logger *zap.Logger, cfg *Config, settings receiver.Settings) *sparkScraper {
	return &sparkScraper{
		logger:   logger,
		config:   cfg,
		settings: settings.TelemetrySettings,
		mb:       metadata.NewMetricsBuilder(cfg.MetricsBuilderConfig, settings),
	}
}

func (s *sparkScraper) start(ctx context.Context, host component.Host) (err error) {
	httpClient, err := newApacheSparkClient(ctx, s.config, host, s.settings)
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
		return pmetric.NewMetrics(), errors.Join(errFailedAppIDCollection, err)
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
	if stat, ok := clusterStats.Gauges[appID+".driver.BlockManager.disk.diskSpaceUsed_MB"]; ok {
		s.mb.RecordSparkDriverBlockManagerDiskUsageDataPoint(now, int64(stat.Value))
	}
	if stat, ok := clusterStats.Gauges[appID+".driver.BlockManager.memory.offHeapMemUsed_MB"]; ok {
		s.mb.RecordSparkDriverBlockManagerMemoryUsageDataPoint(now, int64(stat.Value), metadata.AttributeLocationOffHeap, metadata.AttributeStateUsed)
	}
	if stat, ok := clusterStats.Gauges[appID+".driver.BlockManager.memory.onHeapMemUsed_MB"]; ok {
		s.mb.RecordSparkDriverBlockManagerMemoryUsageDataPoint(now, int64(stat.Value), metadata.AttributeLocationOnHeap, metadata.AttributeStateUsed)
	}
	if stat, ok := clusterStats.Gauges[appID+".driver.BlockManager.memory.remainingOffHeapMem_MB"]; ok {
		s.mb.RecordSparkDriverBlockManagerMemoryUsageDataPoint(now, int64(stat.Value), metadata.AttributeLocationOffHeap, metadata.AttributeStateFree)
	}
	if stat, ok := clusterStats.Gauges[appID+".driver.BlockManager.memory.remainingOnHeapMem_MB"]; ok {
		s.mb.RecordSparkDriverBlockManagerMemoryUsageDataPoint(now, int64(stat.Value), metadata.AttributeLocationOnHeap, metadata.AttributeStateFree)
	}

	if stat, ok := clusterStats.Counters[appID+".driver.HiveExternalCatalog.fileCacheHits"]; ok {
		s.mb.RecordSparkDriverHiveExternalCatalogFileCacheHitsDataPoint(now, stat.Count)
	}
	if stat, ok := clusterStats.Counters[appID+".driver.HiveExternalCatalog.filesDiscovered"]; ok {
		s.mb.RecordSparkDriverHiveExternalCatalogFilesDiscoveredDataPoint(now, stat.Count)
	}
	if stat, ok := clusterStats.Counters[appID+".driver.HiveExternalCatalog.hiveClientCalls"]; ok {
		s.mb.RecordSparkDriverHiveExternalCatalogHiveClientCallsDataPoint(now, stat.Count)
	}
	if stat, ok := clusterStats.Counters[appID+".driver.HiveExternalCatalog.parallelListingJobCount"]; ok {
		s.mb.RecordSparkDriverHiveExternalCatalogParallelListingJobsDataPoint(now, stat.Count)
	}
	if stat, ok := clusterStats.Counters[appID+".driver.HiveExternalCatalog.partitionsFetched"]; ok {
		s.mb.RecordSparkDriverHiveExternalCatalogPartitionsFetchedDataPoint(now, stat.Count)
	}

	if stat, ok := clusterStats.Histograms[appID+".driver.CodeGenerator.compilationTime"]; ok {
		s.mb.RecordSparkDriverCodeGeneratorCompilationCountDataPoint(now, stat.Count)
		s.mb.RecordSparkDriverCodeGeneratorCompilationAverageTimeDataPoint(now, stat.Mean)
	}
	if stat, ok := clusterStats.Histograms[appID+".driver.CodeGenerator.generatedClassSize"]; ok {
		s.mb.RecordSparkDriverCodeGeneratorGeneratedClassCountDataPoint(now, stat.Count)
		s.mb.RecordSparkDriverCodeGeneratorGeneratedClassAverageSizeDataPoint(now, stat.Mean)
	}
	if stat, ok := clusterStats.Histograms[appID+".driver.CodeGenerator.generatedMethodSize"]; ok {
		s.mb.RecordSparkDriverCodeGeneratorGeneratedMethodCountDataPoint(now, stat.Count)
		s.mb.RecordSparkDriverCodeGeneratorGeneratedMethodAverageSizeDataPoint(now, stat.Mean)
	}
	if stat, ok := clusterStats.Histograms[appID+".driver.CodeGenerator.sourceCodeSize"]; ok {
		s.mb.RecordSparkDriverCodeGeneratorSourceCodeOperationsDataPoint(now, stat.Count)
		s.mb.RecordSparkDriverCodeGeneratorSourceCodeAverageSizeDataPoint(now, stat.Mean)
	}

	if stat, ok := clusterStats.Gauges[appID+".driver.DAGScheduler.job.activeJobs"]; ok {
		s.mb.RecordSparkDriverDagSchedulerJobActiveDataPoint(now, int64(stat.Value))
	}
	if stat, ok := clusterStats.Gauges[appID+".driver.DAGScheduler.job.allJobs"]; ok {
		s.mb.RecordSparkDriverDagSchedulerJobCountDataPoint(now, int64(stat.Value))
	}
	if stat, ok := clusterStats.Gauges[appID+".driver.DAGScheduler.stage.failedStages"]; ok {
		s.mb.RecordSparkDriverDagSchedulerStageFailedDataPoint(now, int64(stat.Value))
	}
	if stat, ok := clusterStats.Gauges[appID+".driver.DAGScheduler.stage.runningStages"]; ok {
		s.mb.RecordSparkDriverDagSchedulerStageCountDataPoint(now, int64(stat.Value), metadata.AttributeSchedulerStatusRunning)
	}
	if stat, ok := clusterStats.Gauges[appID+".driver.DAGScheduler.stage.waitingStages"]; ok {
		s.mb.RecordSparkDriverDagSchedulerStageCountDataPoint(now, int64(stat.Value), metadata.AttributeSchedulerStatusWaiting)
	}

	if stat, ok := clusterStats.Counters[appID+".driver.LiveListenerBus.numEventsPosted"]; ok {
		s.mb.RecordSparkDriverLiveListenerBusPostedDataPoint(now, stat.Count)
	}
	if stat, ok := clusterStats.Timers[appID+".driver.LiveListenerBus.queue.appStatus.listenerProcessingTime"]; ok {
		s.mb.RecordSparkDriverLiveListenerBusProcessingTimeAverageDataPoint(now, stat.Mean)
	}
	if stat, ok := clusterStats.Counters[appID+".driver.LiveListenerBus.queue.appStatus.numDroppedEvents"]; ok {
		s.mb.RecordSparkDriverLiveListenerBusDroppedDataPoint(now, stat.Count)
	}
	if stat, ok := clusterStats.Gauges[appID+".driver.LiveListenerBus.queue.appStatus.size"]; ok {
		s.mb.RecordSparkDriverLiveListenerBusQueueSizeDataPoint(now, int64(stat.Value))
	}

	if stat, ok := clusterStats.Gauges[appID+".driver.JVMCPU.jvmCpuTime"]; ok {
		s.mb.RecordSparkDriverJvmCPUTimeDataPoint(now, int64(stat.Value))
	}

	if stat, ok := clusterStats.Gauges[appID+".driver.ExecutorMetrics.JVMOffHeapMemory"]; ok {
		s.mb.RecordSparkDriverExecutorMemoryJvmDataPoint(now, int64(stat.Value), metadata.AttributeLocationOffHeap)
	}
	if stat, ok := clusterStats.Gauges[appID+".driver.ExecutorMetrics.JVMHeapMemory"]; ok {
		s.mb.RecordSparkDriverExecutorMemoryJvmDataPoint(now, int64(stat.Value), metadata.AttributeLocationOnHeap)
	}
	if stat, ok := clusterStats.Gauges[appID+".driver.ExecutorMetrics.OffHeapExecutionMemory"]; ok {
		s.mb.RecordSparkDriverExecutorMemoryExecutionDataPoint(now, int64(stat.Value), metadata.AttributeLocationOffHeap)
	}
	if stat, ok := clusterStats.Gauges[appID+".driver.ExecutorMetrics.OnHeapExecutionMemory"]; ok {
		s.mb.RecordSparkDriverExecutorMemoryExecutionDataPoint(now, int64(stat.Value), metadata.AttributeLocationOnHeap)
	}
	if stat, ok := clusterStats.Gauges[appID+".driver.ExecutorMetrics.OffHeapStorageMemory"]; ok {
		s.mb.RecordSparkDriverExecutorMemoryStorageDataPoint(now, int64(stat.Value), metadata.AttributeLocationOffHeap)
	}
	if stat, ok := clusterStats.Gauges[appID+".driver.ExecutorMetrics.OnHeapStorageMemory"]; ok {
		s.mb.RecordSparkDriverExecutorMemoryStorageDataPoint(now, int64(stat.Value), metadata.AttributeLocationOnHeap)
	}
	if stat, ok := clusterStats.Gauges[appID+".driver.ExecutorMetrics.DirectPoolMemory"]; ok {
		s.mb.RecordSparkDriverExecutorMemoryPoolDataPoint(now, int64(stat.Value), metadata.AttributePoolMemoryTypeDirect)
	}
	if stat, ok := clusterStats.Gauges[appID+".driver.ExecutorMetrics.MappedPoolMemory"]; ok {
		s.mb.RecordSparkDriverExecutorMemoryPoolDataPoint(now, int64(stat.Value), metadata.AttributePoolMemoryTypeMapped)
	}
	if stat, ok := clusterStats.Gauges[appID+".driver.ExecutorMetrics.MinorGCCount"]; ok {
		s.mb.RecordSparkDriverExecutorGcOperationsDataPoint(now, int64(stat.Value), metadata.AttributeGcTypeMinor)
	}
	if stat, ok := clusterStats.Gauges[appID+".driver.ExecutorMetrics.MajorGCCount"]; ok {
		s.mb.RecordSparkDriverExecutorGcOperationsDataPoint(now, int64(stat.Value), metadata.AttributeGcTypeMajor)
	}
	if stat, ok := clusterStats.Gauges[appID+".driver.ExecutorMetrics.MinorGCTime"]; ok {
		s.mb.RecordSparkDriverExecutorGcTimeDataPoint(now, int64(stat.Value), metadata.AttributeGcTypeMinor)
	}
	if stat, ok := clusterStats.Gauges[appID+".driver.ExecutorMetrics.MajorGCTime"]; ok {
		s.mb.RecordSparkDriverExecutorGcTimeDataPoint(now, int64(stat.Value), metadata.AttributeGcTypeMajor)
	}

	rb := s.mb.NewResourceBuilder()
	rb.SetSparkApplicationID(appID)
	rb.SetSparkApplicationName(appName)
	s.mb.EmitForResource(metadata.WithResource(rb.Emit()))
}

func (s *sparkScraper) recordStages(stageStats []models.Stage, now pcommon.Timestamp, appID string, appName string) {
	for _, stage := range stageStats {
		switch stage.Status {
		case "ACTIVE":
			s.mb.RecordSparkStageStatusDataPoint(now, 0, true, false, false, false)
		case "COMPLETE":
			s.mb.RecordSparkStageStatusDataPoint(now, 0, false, true, false, false)
		case "PENDING":
			s.mb.RecordSparkStageStatusDataPoint(now, 0, false, false, true, false)
		case "FAILED":
			s.mb.RecordSparkStageStatusDataPoint(now, 0, false, false, false, true)
		default:
			s.logger.Warn("Unsupported Spark stage status supplied: ignoring this stage's metrics and continuing to metrics for next stage", zap.String("status", stage.Status))
			continue
		}

		s.mb.RecordSparkStageTaskActiveDataPoint(now, stage.NumActiveTasks)
		s.mb.RecordSparkStageTaskResultDataPoint(now, stage.NumCompleteTasks, metadata.AttributeStageTaskResultCompleted)
		s.mb.RecordSparkStageTaskResultDataPoint(now, stage.NumFailedTasks, metadata.AttributeStageTaskResultFailed)
		s.mb.RecordSparkStageTaskResultDataPoint(now, stage.NumKilledTasks, metadata.AttributeStageTaskResultKilled)
		s.mb.RecordSparkStageExecutorRunTimeDataPoint(now, stage.ExecutorRunTime)
		s.mb.RecordSparkStageExecutorCPUTimeDataPoint(now, stage.ExecutorCPUTime)
		s.mb.RecordSparkStageTaskResultSizeDataPoint(now, stage.ResultSize)
		s.mb.RecordSparkStageJvmGcTimeDataPoint(now, stage.JvmGcTime)
		s.mb.RecordSparkStageMemorySpilledDataPoint(now, stage.MemoryBytesSpilled)
		s.mb.RecordSparkStageDiskSpilledDataPoint(now, stage.DiskBytesSpilled)
		s.mb.RecordSparkStageMemoryPeakDataPoint(now, stage.PeakExecutionMemory)
		s.mb.RecordSparkStageIoSizeDataPoint(now, stage.InputBytes, metadata.AttributeDirectionIn)
		s.mb.RecordSparkStageIoSizeDataPoint(now, stage.OutputBytes, metadata.AttributeDirectionOut)
		s.mb.RecordSparkStageIoRecordsDataPoint(now, stage.InputRecords, metadata.AttributeDirectionIn)
		s.mb.RecordSparkStageIoRecordsDataPoint(now, stage.OutputRecords, metadata.AttributeDirectionOut)
		s.mb.RecordSparkStageShuffleBlocksFetchedDataPoint(now, stage.ShuffleRemoteBlocksFetched, metadata.AttributeSourceRemote)
		s.mb.RecordSparkStageShuffleBlocksFetchedDataPoint(now, stage.ShuffleLocalBlocksFetched, metadata.AttributeSourceLocal)
		s.mb.RecordSparkStageShuffleFetchWaitTimeDataPoint(now, stage.ShuffleFetchWaitTime)
		s.mb.RecordSparkStageShuffleIoDiskDataPoint(now, stage.ShuffleRemoteBytesReadToDisk)
		s.mb.RecordSparkStageShuffleIoReadSizeDataPoint(now, stage.ShuffleLocalBytesRead, metadata.AttributeSourceLocal)
		s.mb.RecordSparkStageShuffleIoReadSizeDataPoint(now, stage.ShuffleRemoteBytesRead, metadata.AttributeSourceRemote)
		s.mb.RecordSparkStageShuffleIoWriteSizeDataPoint(now, stage.ShuffleWriteBytes)
		s.mb.RecordSparkStageShuffleIoRecordsDataPoint(now, stage.ShuffleReadRecords, metadata.AttributeDirectionIn)
		s.mb.RecordSparkStageShuffleIoRecordsDataPoint(now, stage.ShuffleWriteRecords, metadata.AttributeDirectionOut)
		s.mb.RecordSparkStageShuffleWriteTimeDataPoint(now, stage.ShuffleWriteTime)

		rb := s.mb.NewResourceBuilder()
		rb.SetSparkApplicationID(appID)
		rb.SetSparkApplicationName(appName)
		rb.SetSparkStageID(stage.StageID)
		rb.SetSparkStageAttemptID(stage.AttemptID)
		s.mb.EmitForResource(metadata.WithResource(rb.Emit()))
	}
}

func (s *sparkScraper) recordExecutors(executorStats []models.Executor, now pcommon.Timestamp, appID string, appName string) {
	for _, executor := range executorStats {
		s.mb.RecordSparkExecutorMemoryUsageDataPoint(now, executor.MemoryUsed)
		s.mb.RecordSparkExecutorDiskUsageDataPoint(now, executor.DiskUsed)
		s.mb.RecordSparkExecutorTaskLimitDataPoint(now, executor.MaxTasks)
		s.mb.RecordSparkExecutorTaskActiveDataPoint(now, executor.ActiveTasks)
		s.mb.RecordSparkExecutorTaskResultDataPoint(now, executor.FailedTasks, metadata.AttributeExecutorTaskResultFailed)
		s.mb.RecordSparkExecutorTaskResultDataPoint(now, executor.CompletedTasks, metadata.AttributeExecutorTaskResultCompleted)
		s.mb.RecordSparkExecutorTimeDataPoint(now, executor.TotalDuration)
		s.mb.RecordSparkExecutorGcTimeDataPoint(now, executor.TotalGCTime)
		s.mb.RecordSparkExecutorInputSizeDataPoint(now, executor.TotalInputBytes)
		s.mb.RecordSparkExecutorShuffleIoSizeDataPoint(now, executor.TotalShuffleRead, metadata.AttributeDirectionIn)
		s.mb.RecordSparkExecutorShuffleIoSizeDataPoint(now, executor.TotalShuffleWrite, metadata.AttributeDirectionOut)
		used := executor.UsedOnHeapStorageMemory
		s.mb.RecordSparkExecutorStorageMemoryUsageDataPoint(now, used, metadata.AttributeLocationOnHeap, metadata.AttributeStateUsed)
		s.mb.RecordSparkExecutorStorageMemoryUsageDataPoint(now, executor.TotalOnHeapStorageMemory-used, metadata.AttributeLocationOnHeap, metadata.AttributeStateFree)
		used = executor.UsedOffHeapStorageMemory
		s.mb.RecordSparkExecutorStorageMemoryUsageDataPoint(now, used, metadata.AttributeLocationOffHeap, metadata.AttributeStateUsed)
		s.mb.RecordSparkExecutorStorageMemoryUsageDataPoint(now, executor.TotalOffHeapStorageMemory-used, metadata.AttributeLocationOffHeap, metadata.AttributeStateFree)

		rb := s.mb.NewResourceBuilder()
		rb.SetSparkApplicationID(appID)
		rb.SetSparkApplicationName(appName)
		rb.SetSparkExecutorID(executor.ExecutorID)
		s.mb.EmitForResource(metadata.WithResource(rb.Emit()))
	}
}

func (s *sparkScraper) recordJobs(jobStats []models.Job, now pcommon.Timestamp, appID string, appName string) {
	for _, job := range jobStats {
		s.mb.RecordSparkJobTaskActiveDataPoint(now, job.NumActiveTasks)
		s.mb.RecordSparkJobTaskResultDataPoint(now, job.NumCompletedTasks, metadata.AttributeJobResultCompleted)
		s.mb.RecordSparkJobTaskResultDataPoint(now, job.NumSkippedTasks, metadata.AttributeJobResultSkipped)
		s.mb.RecordSparkJobTaskResultDataPoint(now, job.NumFailedTasks, metadata.AttributeJobResultFailed)
		s.mb.RecordSparkJobStageActiveDataPoint(now, job.NumActiveStages)
		s.mb.RecordSparkJobStageResultDataPoint(now, job.NumCompletedStages, metadata.AttributeJobResultCompleted)
		s.mb.RecordSparkJobStageResultDataPoint(now, job.NumSkippedStages, metadata.AttributeJobResultSkipped)
		s.mb.RecordSparkJobStageResultDataPoint(now, job.NumFailedStages, metadata.AttributeJobResultFailed)

		rb := s.mb.NewResourceBuilder()
		rb.SetSparkApplicationID(appID)
		rb.SetSparkApplicationName(appName)
		rb.SetSparkJobID(job.JobID)
		s.mb.EmitForResource(metadata.WithResource(rb.Emit()))
	}
}
