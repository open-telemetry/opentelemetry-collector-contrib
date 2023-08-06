// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package elasticsearchreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/elasticsearchreceiver"

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/hashicorp/go-version"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/scrapererror"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/elasticsearchreceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/elasticsearchreceiver/internal/model"
)

var (
	es7_10 = func() *version.Version {
		v, _ := version.NewVersion("7.10")
		return v
	}()
	es7_13 = func() *version.Version {
		v, _ := version.NewVersion("7.13")
		return v
	}()

	_ = featuregate.GlobalRegistry().MustRegister(
		"receiver.elasticsearch.emitNodeVersionAttr",
		featuregate.StageStable,
		featuregate.WithRegisterToVersion("0.82.0"),
		featuregate.WithRegisterDescription("All node metrics will be enriched with the node version resource attribute."),
		featuregate.WithRegisterReferenceURL("https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/16847"),
	)
)

var errUnknownClusterStatus = errors.New("unknown cluster status")

type elasticsearchScraper struct {
	client      elasticsearchClient
	settings    component.TelemetrySettings
	cfg         *Config
	mb          *metadata.MetricsBuilder
	version     *version.Version
	clusterName string
}

func newElasticSearchScraper(
	settings receiver.CreateSettings,
	cfg *Config,
) *elasticsearchScraper {
	return &elasticsearchScraper{
		settings: settings.TelemetrySettings,
		cfg:      cfg,
		mb:       metadata.NewMetricsBuilder(cfg.MetricsBuilderConfig, settings),
	}
}

func (r *elasticsearchScraper) start(_ context.Context, host component.Host) (err error) {
	r.client, err = newElasticsearchClient(r.settings, *r.cfg, host)
	return
}

func (r *elasticsearchScraper) scrape(ctx context.Context) (pmetric.Metrics, error) {
	errs := &scrapererror.ScrapeErrors{}

	now := pcommon.NewTimestampFromTime(time.Now())

	r.getClusterMetadata(ctx, errs)
	r.scrapeNodeMetrics(ctx, now, errs)
	r.scrapeClusterMetrics(ctx, now, errs)
	r.scrapeIndicesMetrics(ctx, now, errs)

	return r.mb.Emit(), errs.Combine()
}

// scrapeVersion gets and assigns the elasticsearch version number
func (r *elasticsearchScraper) getClusterMetadata(ctx context.Context, errs *scrapererror.ScrapeErrors) {
	response, err := r.client.ClusterMetadata(ctx)
	if err != nil {
		errs.AddPartial(2, err)
		return
	}

	r.clusterName = response.ClusterName

	esVersion, err := version.NewVersion(response.Version.Number)
	if err != nil {
		errs.AddPartial(2, err)
		return
	}

	r.version = esVersion
}

// scrapeNodeMetrics scrapes adds node-level metrics to the given MetricSlice from the NodeStats endpoint
func (r *elasticsearchScraper) scrapeNodeMetrics(ctx context.Context, now pcommon.Timestamp, errs *scrapererror.ScrapeErrors) {
	if len(r.cfg.Nodes) == 0 {
		return
	}

	nodeStats, err := r.client.NodeStats(ctx, r.cfg.Nodes)
	if err != nil {
		errs.AddPartial(26, err)
		return
	}

	// Certain node metadata is not available from the /_nodes/stats endpoint. Therefore, we need to get this metadata
	// from the /_nodes endpoint.
	nodesInfo, err := r.client.Nodes(ctx, r.cfg.Nodes)
	if err != nil {
		errs.AddPartial(26, err)
		return
	}

	for id, info := range nodeStats.Nodes {
		rb := r.mb.NewResourceBuilder()
		rb.SetElasticsearchClusterName(nodeStats.ClusterName)
		rb.SetElasticsearchNodeName(info.Name)
		if node, ok := nodesInfo.Nodes[id]; ok {
			rb.SetElasticsearchNodeVersion(node.Version)
		}
		rmb := r.mb.ResourceMetricsBuilder(rb.Emit())

		rmb.RecordElasticsearchNodeCacheMemoryUsageDataPoint(now, info.Indices.FieldDataCache.MemorySizeInBy, metadata.AttributeCacheNameFielddata)
		rmb.RecordElasticsearchNodeCacheMemoryUsageDataPoint(now, info.Indices.QueryCache.MemorySizeInBy, metadata.AttributeCacheNameQuery)

		rmb.RecordElasticsearchNodeCacheEvictionsDataPoint(now, info.Indices.FieldDataCache.Evictions, metadata.AttributeCacheNameFielddata)
		rmb.RecordElasticsearchNodeCacheEvictionsDataPoint(now, info.Indices.QueryCache.Evictions, metadata.AttributeCacheNameQuery)

		rmb.RecordElasticsearchNodeCacheCountDataPoint(now, info.Indices.QueryCache.HitCount, metadata.AttributeQueryCacheCountTypeHit)
		rmb.RecordElasticsearchNodeCacheCountDataPoint(now, info.Indices.QueryCache.MissCount, metadata.AttributeQueryCacheCountTypeMiss)

		rmb.RecordElasticsearchNodeCacheSizeDataPoint(now, info.Indices.QueryCache.MemorySizeInBy)

		rmb.RecordElasticsearchNodeFsDiskAvailableDataPoint(now, info.FS.Total.AvailableBytes)
		rmb.RecordElasticsearchNodeFsDiskFreeDataPoint(now, info.FS.Total.FreeBytes)
		rmb.RecordElasticsearchNodeFsDiskTotalDataPoint(now, info.FS.Total.TotalBytes)

		rmb.RecordElasticsearchNodeDiskIoReadDataPoint(now, info.FS.IOStats.Total.ReadBytes)
		rmb.RecordElasticsearchNodeDiskIoWriteDataPoint(now, info.FS.IOStats.Total.WriteBytes)

		rmb.RecordElasticsearchNodeClusterIoDataPoint(now, info.TransportStats.ReceivedBytes, metadata.AttributeDirectionReceived)
		rmb.RecordElasticsearchNodeClusterIoDataPoint(now, info.TransportStats.SentBytes, metadata.AttributeDirectionSent)

		rmb.RecordElasticsearchNodeClusterConnectionsDataPoint(now, info.TransportStats.OpenConnections)

		rmb.RecordElasticsearchNodeHTTPConnectionsDataPoint(now, info.HTTPStats.OpenConnections)

		rmb.RecordElasticsearchNodeOperationsCurrentDataPoint(now, info.Indices.SearchOperations.QueryCurrent, metadata.AttributeOperationQuery)

		rmb.RecordElasticsearchNodeOperationsCompletedDataPoint(now, info.Indices.IndexingOperations.IndexTotal, metadata.AttributeOperationIndex)
		rmb.RecordElasticsearchNodeOperationsCompletedDataPoint(now, info.Indices.IndexingOperations.DeleteTotal, metadata.AttributeOperationDelete)
		rmb.RecordElasticsearchNodeOperationsCompletedDataPoint(now, info.Indices.GetOperation.Total, metadata.AttributeOperationGet)
		rmb.RecordElasticsearchNodeOperationsCompletedDataPoint(now, info.Indices.SearchOperations.QueryTotal, metadata.AttributeOperationQuery)
		rmb.RecordElasticsearchNodeOperationsCompletedDataPoint(now, info.Indices.SearchOperations.FetchTotal, metadata.AttributeOperationFetch)
		rmb.RecordElasticsearchNodeOperationsCompletedDataPoint(now, info.Indices.SearchOperations.ScrollTotal, metadata.AttributeOperationScroll)
		rmb.RecordElasticsearchNodeOperationsCompletedDataPoint(now, info.Indices.SearchOperations.SuggestTotal, metadata.AttributeOperationSuggest)
		rmb.RecordElasticsearchNodeOperationsCompletedDataPoint(now, info.Indices.MergeOperations.Total, metadata.AttributeOperationMerge)
		rmb.RecordElasticsearchNodeOperationsCompletedDataPoint(now, info.Indices.RefreshOperations.Total, metadata.AttributeOperationRefresh)
		rmb.RecordElasticsearchNodeOperationsCompletedDataPoint(now, info.Indices.FlushOperations.Total, metadata.AttributeOperationFlush)
		rmb.RecordElasticsearchNodeOperationsCompletedDataPoint(now, info.Indices.WarmerOperations.Total, metadata.AttributeOperationWarmer)

		rmb.RecordElasticsearchNodeOperationsTimeDataPoint(now, info.Indices.IndexingOperations.IndexTimeInMs, metadata.AttributeOperationIndex)
		rmb.RecordElasticsearchNodeOperationsTimeDataPoint(now, info.Indices.IndexingOperations.DeleteTimeInMs, metadata.AttributeOperationDelete)
		rmb.RecordElasticsearchNodeOperationsTimeDataPoint(now, info.Indices.GetOperation.TotalTimeInMs, metadata.AttributeOperationGet)
		rmb.RecordElasticsearchNodeOperationsTimeDataPoint(now, info.Indices.SearchOperations.QueryTimeInMs, metadata.AttributeOperationQuery)
		rmb.RecordElasticsearchNodeOperationsTimeDataPoint(now, info.Indices.SearchOperations.FetchTimeInMs, metadata.AttributeOperationFetch)
		rmb.RecordElasticsearchNodeOperationsTimeDataPoint(now, info.Indices.SearchOperations.ScrollTimeInMs, metadata.AttributeOperationScroll)
		rmb.RecordElasticsearchNodeOperationsTimeDataPoint(now, info.Indices.SearchOperations.SuggestTimeInMs, metadata.AttributeOperationSuggest)
		rmb.RecordElasticsearchNodeOperationsTimeDataPoint(now, info.Indices.MergeOperations.TotalTimeInMs, metadata.AttributeOperationMerge)
		rmb.RecordElasticsearchNodeOperationsTimeDataPoint(now, info.Indices.RefreshOperations.TotalTimeInMs, metadata.AttributeOperationRefresh)
		rmb.RecordElasticsearchNodeOperationsTimeDataPoint(now, info.Indices.FlushOperations.TotalTimeInMs, metadata.AttributeOperationFlush)
		rmb.RecordElasticsearchNodeOperationsTimeDataPoint(now, info.Indices.WarmerOperations.TotalTimeInMs, metadata.AttributeOperationWarmer)

		rmb.RecordElasticsearchNodeOperationsGetCompletedDataPoint(now, info.Indices.GetOperation.Exists, metadata.AttributeGetResultHit)
		rmb.RecordElasticsearchNodeOperationsGetCompletedDataPoint(now, info.Indices.GetOperation.Missing, metadata.AttributeGetResultMiss)

		rmb.RecordElasticsearchNodeOperationsGetTimeDataPoint(now, info.Indices.GetOperation.ExistsTimeInMs, metadata.AttributeGetResultHit)
		rmb.RecordElasticsearchNodeOperationsGetTimeDataPoint(now, info.Indices.GetOperation.MissingTimeInMs, metadata.AttributeGetResultMiss)

		rmb.RecordElasticsearchNodeShardsSizeDataPoint(now, info.Indices.StoreInfo.SizeInBy)

		// Elasticsearch version 7.13+ is required to collect `elasticsearch.node.shards.data_set.size`.
		// Reference: https://github.com/elastic/elasticsearch/pull/70625/files#diff-354b5b1f25978b5c638cb707622ae79b42b40aace6f27f3f9d5dd1e31e67b1caR7
		if r.version != nil && r.version.GreaterThanOrEqual(es7_13) {
			rmb.RecordElasticsearchNodeShardsDataSetSizeDataPoint(now, info.Indices.StoreInfo.DataSetSizeInBy)
		}

		rmb.RecordElasticsearchNodeShardsReservedSizeDataPoint(now, info.Indices.StoreInfo.ReservedInBy)

		for tpName, tpInfo := range info.ThreadPoolInfo {
			rmb.RecordElasticsearchNodeThreadPoolThreadsDataPoint(now, tpInfo.ActiveThreads, tpName, metadata.AttributeThreadStateActive)
			rmb.RecordElasticsearchNodeThreadPoolThreadsDataPoint(now, tpInfo.TotalThreads-tpInfo.ActiveThreads, tpName, metadata.AttributeThreadStateIdle)

			rmb.RecordElasticsearchNodeThreadPoolTasksQueuedDataPoint(now, tpInfo.QueuedTasks, tpName)

			rmb.RecordElasticsearchNodeThreadPoolTasksFinishedDataPoint(now, tpInfo.CompletedTasks, tpName, metadata.AttributeTaskStateCompleted)
			rmb.RecordElasticsearchNodeThreadPoolTasksFinishedDataPoint(now, tpInfo.RejectedTasks, tpName, metadata.AttributeTaskStateRejected)
		}

		for cbName, cbInfo := range info.CircuitBreakerInfo {
			rmb.RecordElasticsearchBreakerMemoryEstimatedDataPoint(now, cbInfo.EstimatedSizeInBytes, cbName)
			rmb.RecordElasticsearchBreakerMemoryLimitDataPoint(now, cbInfo.LimitSizeInBytes, cbName)
			rmb.RecordElasticsearchBreakerTrippedDataPoint(now, cbInfo.Tripped, cbName)
		}

		rmb.RecordElasticsearchNodeDocumentsDataPoint(now, info.Indices.DocumentStats.ActiveCount, metadata.AttributeDocumentStateActive)
		rmb.RecordElasticsearchNodeDocumentsDataPoint(now, info.Indices.DocumentStats.DeletedCount, metadata.AttributeDocumentStateDeleted)

		rmb.RecordElasticsearchNodeOpenFilesDataPoint(now, info.ProcessStats.OpenFileDescriptorsCount)

		rmb.RecordElasticsearchNodeTranslogOperationsDataPoint(now, info.Indices.TranslogStats.Operations)
		rmb.RecordElasticsearchNodeTranslogSizeDataPoint(now, info.Indices.TranslogStats.SizeInBy)
		rmb.RecordElasticsearchNodeTranslogUncommittedSizeDataPoint(now, info.Indices.TranslogStats.UncommittedOperationsInBy)

		rmb.RecordElasticsearchOsCPUUsageDataPoint(now, info.OS.CPU.Usage)
		rmb.RecordElasticsearchOsCPULoadAvg1mDataPoint(now, info.OS.CPU.LoadAvg.OneMinute)
		rmb.RecordElasticsearchOsCPULoadAvg5mDataPoint(now, info.OS.CPU.LoadAvg.FiveMinutes)
		rmb.RecordElasticsearchOsCPULoadAvg15mDataPoint(now, info.OS.CPU.LoadAvg.FifteenMinutes)

		// Elasticsearch sends this data in percent, but we want to represent it as a number between 0 and 1, so we need to divide.
		// Additionally, if the usage is not known, ES will send '-1'. We do not want to report the metric in this case.
		if info.ProcessStats.CPU.Percent != -1 {
			rmb.RecordElasticsearchProcessCPUUsageDataPoint(now, float64(info.ProcessStats.CPU.Percent)/100)
		}
		if info.ProcessStats.CPU.TotalInMs != -1 {
			rmb.RecordElasticsearchProcessCPUTimeDataPoint(now, info.ProcessStats.CPU.TotalInMs)
		}

		rmb.RecordElasticsearchProcessMemoryVirtualDataPoint(now, info.ProcessStats.Memory.TotalVirtualInBy)

		rmb.RecordElasticsearchOsMemoryDataPoint(now, info.OS.Memory.UsedInBy, metadata.AttributeMemoryStateUsed)
		rmb.RecordElasticsearchOsMemoryDataPoint(now, info.OS.Memory.FreeInBy, metadata.AttributeMemoryStateFree)

		rmb.RecordJvmClassesLoadedDataPoint(now, info.JVMInfo.ClassInfo.CurrentLoadedCount)

		rmb.RecordJvmGcCollectionsCountDataPoint(now, info.JVMInfo.JVMGCInfo.Collectors.Young.CollectionCount, "young")
		rmb.RecordJvmGcCollectionsCountDataPoint(now, info.JVMInfo.JVMGCInfo.Collectors.Old.CollectionCount, "old")

		rmb.RecordJvmGcCollectionsElapsedDataPoint(now, info.JVMInfo.JVMGCInfo.Collectors.Young.CollectionTimeInMillis, "young")
		rmb.RecordJvmGcCollectionsElapsedDataPoint(now, info.JVMInfo.JVMGCInfo.Collectors.Old.CollectionTimeInMillis, "old")

		rmb.RecordJvmMemoryHeapMaxDataPoint(now, info.JVMInfo.JVMMemoryInfo.MaxHeapInBy)
		rmb.RecordJvmMemoryHeapUsedDataPoint(now, info.JVMInfo.JVMMemoryInfo.HeapUsedInBy)
		rmb.RecordJvmMemoryHeapCommittedDataPoint(now, info.JVMInfo.JVMMemoryInfo.HeapCommittedInBy)
		// Elasticsearch sends this data in percent, but we want to represent it as a number between 0 and 1, so we need to divide.
		rmb.RecordJvmMemoryHeapUtilizationDataPoint(now, float64(info.JVMInfo.JVMMemoryInfo.HeapUsedPercent)/100)

		rmb.RecordJvmMemoryNonheapUsedDataPoint(now, info.JVMInfo.JVMMemoryInfo.NonHeapUsedInBy)
		rmb.RecordJvmMemoryNonheapCommittedDataPoint(now, info.JVMInfo.JVMMemoryInfo.NonHeapComittedInBy)

		rmb.RecordJvmMemoryPoolUsedDataPoint(now, info.JVMInfo.JVMMemoryInfo.MemoryPools.Young.MemUsedBy, "young")
		rmb.RecordJvmMemoryPoolUsedDataPoint(now, info.JVMInfo.JVMMemoryInfo.MemoryPools.Survivor.MemUsedBy, "survivor")
		rmb.RecordJvmMemoryPoolUsedDataPoint(now, info.JVMInfo.JVMMemoryInfo.MemoryPools.Old.MemUsedBy, "old")

		rmb.RecordJvmMemoryPoolMaxDataPoint(now, info.JVMInfo.JVMMemoryInfo.MemoryPools.Young.MemMaxBy, "young")
		rmb.RecordJvmMemoryPoolMaxDataPoint(now, info.JVMInfo.JVMMemoryInfo.MemoryPools.Survivor.MemMaxBy, "survivor")
		rmb.RecordJvmMemoryPoolMaxDataPoint(now, info.JVMInfo.JVMMemoryInfo.MemoryPools.Old.MemMaxBy, "old")

		rmb.RecordJvmThreadsCountDataPoint(now, info.JVMInfo.JVMThreadInfo.Count)

		// Elasticsearch version 7.10+ is required to collect `elasticsearch.indexing_pressure.memory.limit`.
		// Reference: https://github.com/elastic/elasticsearch/pull/60342/files#diff-13864344bab3afc267797d67b2746e2939a3fd8af7611ac9fbda376323e2f5eaR37
		if r.version != nil && r.version.GreaterThanOrEqual(es7_10) {
			rmb.RecordElasticsearchIndexingPressureMemoryLimitDataPoint(now, info.IndexingPressure.Memory.LimitInBy)
		}

		rmb.RecordElasticsearchMemoryIndexingPressureDataPoint(now, info.IndexingPressure.Memory.Current.PrimaryInBy, metadata.AttributeIndexingPressureStagePrimary)
		rmb.RecordElasticsearchMemoryIndexingPressureDataPoint(now, info.IndexingPressure.Memory.Current.CoordinatingInBy, metadata.AttributeIndexingPressureStageCoordinating)
		rmb.RecordElasticsearchMemoryIndexingPressureDataPoint(now, info.IndexingPressure.Memory.Current.ReplicaInBy, metadata.AttributeIndexingPressureStageReplica)
		rmb.RecordElasticsearchIndexingPressureMemoryTotalPrimaryRejectionsDataPoint(now, info.IndexingPressure.Memory.Total.PrimaryRejections)
		rmb.RecordElasticsearchIndexingPressureMemoryTotalReplicaRejectionsDataPoint(now, info.IndexingPressure.Memory.Total.ReplicaRejections)

		rmb.RecordElasticsearchClusterStateQueueDataPoint(now, info.Discovery.ClusterStateQueue.Committed, metadata.AttributeClusterStateQueueStateCommitted)
		rmb.RecordElasticsearchClusterStateQueueDataPoint(now, info.Discovery.ClusterStateQueue.Committed, metadata.AttributeClusterStateQueueStatePending)

		rmb.RecordElasticsearchClusterPublishedStatesFullDataPoint(now, info.Discovery.PublishedClusterStates.FullStates)
		rmb.RecordElasticsearchClusterPublishedStatesDifferencesDataPoint(now, info.Discovery.PublishedClusterStates.CompatibleDiffs, metadata.AttributeClusterPublishedDifferenceStateCompatible)
		rmb.RecordElasticsearchClusterPublishedStatesDifferencesDataPoint(now, info.Discovery.PublishedClusterStates.IncompatibleDiffs, metadata.AttributeClusterPublishedDifferenceStateIncompatible)

		for cusState, csuInfo := range info.Discovery.ClusterStateUpdate {
			rmb.RecordElasticsearchClusterStateUpdateCountDataPoint(now, csuInfo.Count, cusState)
			rmb.RecordElasticsearchClusterStateUpdateTimeDataPoint(now, csuInfo.ComputationTimeMillis, cusState, metadata.AttributeClusterStateUpdateTypeComputation)
			rmb.RecordElasticsearchClusterStateUpdateTimeDataPoint(now, csuInfo.NotificationTimeMillis, cusState, metadata.AttributeClusterStateUpdateTypeNotification)
			if cusState == "unchanged" {
				// the node_linux.json payload response for "elasticsearch.cluster.state_update.time" with attributes "unchanged" has 2 attributes "computation_time_millis" and "notification_time_millis".
				// All other metrics for "unchanged" should be skipped to prevent 0 emitted metrics"
				// https://github.com/elastic/elasticsearch/pull/76771/files#diff-8bbfc581d91f9440e53098ea7d7864aeaeac1fc83a714133e4aafe38eba8ed90R2098
				continue
			}
			rmb.RecordElasticsearchClusterStateUpdateTimeDataPoint(now, csuInfo.ContextConstructionTimeMillis, cusState, metadata.AttributeClusterStateUpdateTypeContextConstruction)
			rmb.RecordElasticsearchClusterStateUpdateTimeDataPoint(now, csuInfo.CommitTimeMillis, cusState, metadata.AttributeClusterStateUpdateTypeCommit)
			rmb.RecordElasticsearchClusterStateUpdateTimeDataPoint(now, csuInfo.CompletionTimeMillis, cusState, metadata.AttributeClusterStateUpdateTypeCompletion)
			rmb.RecordElasticsearchClusterStateUpdateTimeDataPoint(now, csuInfo.MasterApplyTimeMillis, cusState, metadata.AttributeClusterStateUpdateTypeMasterApply)
		}

		rmb.RecordElasticsearchNodeIngestDocumentsDataPoint(now, info.Ingest.Total.Count)
		rmb.RecordElasticsearchNodeIngestDocumentsCurrentDataPoint(now, info.Ingest.Total.Current)
		rmb.RecordElasticsearchNodeIngestOperationsFailedDataPoint(now, info.Ingest.Total.Failed)

		for ipName, ipInfo := range info.Ingest.Pipelines {
			rmb.RecordElasticsearchNodePipelineIngestDocumentsPreprocessedDataPoint(now, ipInfo.Count, ipName)
			rmb.RecordElasticsearchNodePipelineIngestOperationsFailedDataPoint(now, ipInfo.Failed, ipName)
			rmb.RecordElasticsearchNodePipelineIngestDocumentsCurrentDataPoint(now, ipInfo.Current, ipName)
		}

		rmb.RecordElasticsearchNodeScriptCacheEvictionsDataPoint(now, info.Script.CacheEvictions)
		rmb.RecordElasticsearchNodeScriptCompilationsDataPoint(now, info.Script.Compilations)
		rmb.RecordElasticsearchNodeScriptCompilationLimitTriggeredDataPoint(now, info.Script.CompilationLimitTriggered)

		rmb.RecordElasticsearchNodeSegmentsMemoryDataPoint(
			now, info.Indices.SegmentsStats.DocumentValuesMemoryInBy, metadata.AttributeSegmentsMemoryObjectTypeDocValue,
		)
		rmb.RecordElasticsearchNodeSegmentsMemoryDataPoint(
			now, info.Indices.SegmentsStats.FixedBitSetMemoryInBy, metadata.AttributeSegmentsMemoryObjectTypeFixedBitSet,
		)
		rmb.RecordElasticsearchNodeSegmentsMemoryDataPoint(
			now, info.Indices.SegmentsStats.IndexWriterMemoryInBy, metadata.AttributeSegmentsMemoryObjectTypeIndexWriter,
		)
		rmb.RecordElasticsearchNodeSegmentsMemoryDataPoint(
			now, info.Indices.SegmentsStats.TermsMemoryInBy, metadata.AttributeSegmentsMemoryObjectTypeTerm,
		)
	}
}

func (r *elasticsearchScraper) scrapeClusterMetrics(ctx context.Context, now pcommon.Timestamp, errs *scrapererror.ScrapeErrors) {
	if r.cfg.SkipClusterMetrics {
		return
	}
	rb := r.mb.NewResourceBuilder()
	rb.SetElasticsearchClusterName(r.clusterName)
	rmb := r.mb.ResourceMetricsBuilder(rb.Emit())

	r.scrapeClusterHealthMetrics(ctx, now, rmb, errs)
	r.scrapeClusterStatsMetrics(ctx, now, rmb, errs)
}

func (r *elasticsearchScraper) scrapeClusterStatsMetrics(ctx context.Context, now pcommon.Timestamp, rmb *metadata.ResourceMetricsBuilder,
	errs *scrapererror.ScrapeErrors) {
	if len(r.cfg.Nodes) == 0 {
		return
	}

	clusterStats, err := r.client.ClusterStats(ctx, r.cfg.Nodes)
	if err != nil {
		errs.AddPartial(3, err)
		return
	}

	rmb.RecordJvmMemoryHeapUsedDataPoint(now, clusterStats.NodesStats.JVMInfo.JVMMemoryInfo.HeapUsedInBy)

	rmb.RecordElasticsearchClusterIndicesCacheEvictionsDataPoint(
		now, clusterStats.IndicesStats.FieldDataCache.Evictions, metadata.AttributeCacheNameFielddata,
	)
	rmb.RecordElasticsearchClusterIndicesCacheEvictionsDataPoint(
		now, clusterStats.IndicesStats.QueryCache.Evictions, metadata.AttributeCacheNameQuery,
	)
}

func (r *elasticsearchScraper) scrapeClusterHealthMetrics(ctx context.Context, now pcommon.Timestamp,
	rmb *metadata.ResourceMetricsBuilder, errs *scrapererror.ScrapeErrors) {
	clusterHealth, err := r.client.ClusterHealth(ctx)
	if err != nil {
		errs.AddPartial(4, err)
		return
	}

	rmb.RecordElasticsearchClusterNodesDataPoint(now, clusterHealth.NodeCount)

	rmb.RecordElasticsearchClusterDataNodesDataPoint(now, clusterHealth.DataNodeCount)

	rmb.RecordElasticsearchClusterShardsDataPoint(now, clusterHealth.ActiveShards, metadata.AttributeShardStateActive)
	rmb.RecordElasticsearchClusterShardsDataPoint(now, clusterHealth.InitializingShards, metadata.AttributeShardStateInitializing)
	rmb.RecordElasticsearchClusterShardsDataPoint(now, clusterHealth.RelocatingShards, metadata.AttributeShardStateRelocating)
	rmb.RecordElasticsearchClusterShardsDataPoint(now, clusterHealth.UnassignedShards, metadata.AttributeShardStateUnassigned)
	rmb.RecordElasticsearchClusterShardsDataPoint(now, clusterHealth.ActivePrimaryShards, metadata.AttributeShardStateActivePrimary)
	rmb.RecordElasticsearchClusterShardsDataPoint(now, clusterHealth.DelayedUnassignedShards, metadata.AttributeShardStateUnassignedDelayed)

	rmb.RecordElasticsearchClusterPendingTasksDataPoint(now, clusterHealth.PendingTasksCount)
	rmb.RecordElasticsearchClusterInFlightFetchDataPoint(now, clusterHealth.InFlightFetchCount)

	switch clusterHealth.Status {
	case "green":
		rmb.RecordElasticsearchClusterHealthDataPoint(now, 1, metadata.AttributeHealthStatusGreen)
		rmb.RecordElasticsearchClusterHealthDataPoint(now, 0, metadata.AttributeHealthStatusYellow)
		rmb.RecordElasticsearchClusterHealthDataPoint(now, 0, metadata.AttributeHealthStatusRed)
	case "yellow":
		rmb.RecordElasticsearchClusterHealthDataPoint(now, 0, metadata.AttributeHealthStatusGreen)
		rmb.RecordElasticsearchClusterHealthDataPoint(now, 1, metadata.AttributeHealthStatusYellow)
		rmb.RecordElasticsearchClusterHealthDataPoint(now, 0, metadata.AttributeHealthStatusRed)
	case "red":
		rmb.RecordElasticsearchClusterHealthDataPoint(now, 0, metadata.AttributeHealthStatusGreen)
		rmb.RecordElasticsearchClusterHealthDataPoint(now, 0, metadata.AttributeHealthStatusYellow)
		rmb.RecordElasticsearchClusterHealthDataPoint(now, 1, metadata.AttributeHealthStatusRed)
	default:
		errs.AddPartial(1, fmt.Errorf("health status %s: %w", clusterHealth.Status, errUnknownClusterStatus))
	}
}

func (r *elasticsearchScraper) scrapeIndicesMetrics(ctx context.Context, now pcommon.Timestamp, errs *scrapererror.ScrapeErrors) {
	if len(r.cfg.Indices) == 0 {
		return
	}

	indexStats, err := r.client.IndexStats(ctx, r.cfg.Indices)

	if err != nil {
		errs.AddPartial(63, err)
		return
	}

	// The metrics for all indices are queried by using "_all" name and hence its the name used for labeling them.
	r.scrapeOneIndexMetrics(now, "_all", &indexStats.All)

	for name, stats := range indexStats.Indices {
		r.scrapeOneIndexMetrics(now, name, stats)
	}
}

func (r *elasticsearchScraper) scrapeOneIndexMetrics(now pcommon.Timestamp, name string, stats *model.IndexStatsIndexInfo) {
	rb := r.mb.NewResourceBuilder()
	rb.SetElasticsearchIndexName(name)
	rb.SetElasticsearchClusterName(r.clusterName)
	rmb := r.mb.ResourceMetricsBuilder(rb.Emit())

	rmb.RecordElasticsearchIndexOperationsCompletedDataPoint(
		now, stats.Total.SearchOperations.FetchTotal, metadata.AttributeOperationFetch, metadata.AttributeIndexAggregationTypeTotal,
	)
	rmb.RecordElasticsearchIndexOperationsCompletedDataPoint(
		now, stats.Total.SearchOperations.QueryTotal, metadata.AttributeOperationQuery, metadata.AttributeIndexAggregationTypeTotal,
	)
	rmb.RecordElasticsearchIndexOperationsCompletedDataPoint(
		now, stats.Total.IndexingOperations.IndexTotal, metadata.AttributeOperationIndex, metadata.AttributeIndexAggregationTypeTotal,
	)
	rmb.RecordElasticsearchIndexOperationsCompletedDataPoint(
		now, stats.Total.IndexingOperations.DeleteTotal, metadata.AttributeOperationDelete, metadata.AttributeIndexAggregationTypeTotal,
	)
	rmb.RecordElasticsearchIndexOperationsCompletedDataPoint(
		now, stats.Total.GetOperation.Total, metadata.AttributeOperationGet, metadata.AttributeIndexAggregationTypeTotal,
	)
	rmb.RecordElasticsearchIndexOperationsCompletedDataPoint(
		now, stats.Total.SearchOperations.ScrollTotal, metadata.AttributeOperationScroll, metadata.AttributeIndexAggregationTypeTotal,
	)
	rmb.RecordElasticsearchIndexOperationsCompletedDataPoint(
		now, stats.Total.SearchOperations.SuggestTotal, metadata.AttributeOperationSuggest, metadata.AttributeIndexAggregationTypeTotal,
	)
	rmb.RecordElasticsearchIndexOperationsCompletedDataPoint(
		now, stats.Total.MergeOperations.Total, metadata.AttributeOperationMerge, metadata.AttributeIndexAggregationTypeTotal,
	)
	rmb.RecordElasticsearchIndexOperationsCompletedDataPoint(
		now, stats.Total.RefreshOperations.Total, metadata.AttributeOperationRefresh, metadata.AttributeIndexAggregationTypeTotal,
	)
	rmb.RecordElasticsearchIndexOperationsCompletedDataPoint(
		now, stats.Total.FlushOperations.Total, metadata.AttributeOperationFlush, metadata.AttributeIndexAggregationTypeTotal,
	)
	rmb.RecordElasticsearchIndexOperationsCompletedDataPoint(
		now, stats.Total.WarmerOperations.Total, metadata.AttributeOperationWarmer, metadata.AttributeIndexAggregationTypeTotal,
	)

	rmb.RecordElasticsearchIndexOperationsCompletedDataPoint(
		now, stats.Primaries.SearchOperations.FetchTotal, metadata.AttributeOperationFetch, metadata.AttributeIndexAggregationTypePrimaryShards,
	)
	rmb.RecordElasticsearchIndexOperationsCompletedDataPoint(
		now, stats.Primaries.SearchOperations.QueryTotal, metadata.AttributeOperationQuery, metadata.AttributeIndexAggregationTypePrimaryShards,
	)
	rmb.RecordElasticsearchIndexOperationsCompletedDataPoint(
		now, stats.Primaries.IndexingOperations.IndexTotal, metadata.AttributeOperationIndex, metadata.AttributeIndexAggregationTypePrimaryShards,
	)
	rmb.RecordElasticsearchIndexOperationsCompletedDataPoint(
		now, stats.Primaries.IndexingOperations.DeleteTotal, metadata.AttributeOperationDelete, metadata.AttributeIndexAggregationTypePrimaryShards,
	)
	rmb.RecordElasticsearchIndexOperationsCompletedDataPoint(
		now, stats.Primaries.GetOperation.Total, metadata.AttributeOperationGet, metadata.AttributeIndexAggregationTypePrimaryShards,
	)
	rmb.RecordElasticsearchIndexOperationsCompletedDataPoint(
		now, stats.Primaries.SearchOperations.ScrollTotal, metadata.AttributeOperationScroll, metadata.AttributeIndexAggregationTypePrimaryShards,
	)
	rmb.RecordElasticsearchIndexOperationsCompletedDataPoint(
		now, stats.Primaries.SearchOperations.SuggestTotal, metadata.AttributeOperationSuggest, metadata.AttributeIndexAggregationTypePrimaryShards,
	)
	rmb.RecordElasticsearchIndexOperationsCompletedDataPoint(
		now, stats.Primaries.MergeOperations.Total, metadata.AttributeOperationMerge, metadata.AttributeIndexAggregationTypePrimaryShards,
	)
	rmb.RecordElasticsearchIndexOperationsCompletedDataPoint(
		now, stats.Primaries.RefreshOperations.Total, metadata.AttributeOperationRefresh, metadata.AttributeIndexAggregationTypePrimaryShards,
	)
	rmb.RecordElasticsearchIndexOperationsCompletedDataPoint(
		now, stats.Primaries.FlushOperations.Total, metadata.AttributeOperationFlush, metadata.AttributeIndexAggregationTypePrimaryShards,
	)
	rmb.RecordElasticsearchIndexOperationsCompletedDataPoint(
		now, stats.Primaries.WarmerOperations.Total, metadata.AttributeOperationWarmer, metadata.AttributeIndexAggregationTypePrimaryShards,
	)

	rmb.RecordElasticsearchIndexOperationsTimeDataPoint(
		now, stats.Total.SearchOperations.FetchTimeInMs, metadata.AttributeOperationFetch, metadata.AttributeIndexAggregationTypeTotal,
	)
	rmb.RecordElasticsearchIndexOperationsTimeDataPoint(
		now, stats.Total.SearchOperations.QueryTimeInMs, metadata.AttributeOperationQuery, metadata.AttributeIndexAggregationTypeTotal,
	)
	rmb.RecordElasticsearchIndexOperationsTimeDataPoint(
		now, stats.Total.IndexingOperations.IndexTimeInMs, metadata.AttributeOperationIndex, metadata.AttributeIndexAggregationTypeTotal,
	)
	rmb.RecordElasticsearchIndexOperationsTimeDataPoint(
		now, stats.Total.IndexingOperations.DeleteTimeInMs, metadata.AttributeOperationDelete, metadata.AttributeIndexAggregationTypeTotal,
	)
	rmb.RecordElasticsearchIndexOperationsTimeDataPoint(
		now, stats.Total.GetOperation.TotalTimeInMs, metadata.AttributeOperationGet, metadata.AttributeIndexAggregationTypeTotal,
	)
	rmb.RecordElasticsearchIndexOperationsTimeDataPoint(
		now, stats.Total.SearchOperations.ScrollTimeInMs, metadata.AttributeOperationScroll, metadata.AttributeIndexAggregationTypeTotal,
	)
	rmb.RecordElasticsearchIndexOperationsTimeDataPoint(
		now, stats.Total.SearchOperations.SuggestTimeInMs, metadata.AttributeOperationSuggest, metadata.AttributeIndexAggregationTypeTotal,
	)
	rmb.RecordElasticsearchIndexOperationsTimeDataPoint(
		now, stats.Total.MergeOperations.TotalTimeInMs, metadata.AttributeOperationMerge, metadata.AttributeIndexAggregationTypeTotal,
	)
	rmb.RecordElasticsearchIndexOperationsTimeDataPoint(
		now, stats.Total.RefreshOperations.TotalTimeInMs, metadata.AttributeOperationRefresh, metadata.AttributeIndexAggregationTypeTotal,
	)
	rmb.RecordElasticsearchIndexOperationsTimeDataPoint(
		now, stats.Total.FlushOperations.TotalTimeInMs, metadata.AttributeOperationFlush, metadata.AttributeIndexAggregationTypeTotal,
	)
	rmb.RecordElasticsearchIndexOperationsTimeDataPoint(
		now, stats.Total.WarmerOperations.TotalTimeInMs, metadata.AttributeOperationWarmer, metadata.AttributeIndexAggregationTypeTotal,
	)

	rmb.RecordElasticsearchIndexOperationsTimeDataPoint(
		now, stats.Primaries.SearchOperations.FetchTimeInMs, metadata.AttributeOperationFetch, metadata.AttributeIndexAggregationTypePrimaryShards,
	)
	rmb.RecordElasticsearchIndexOperationsTimeDataPoint(
		now, stats.Primaries.SearchOperations.QueryTimeInMs, metadata.AttributeOperationQuery, metadata.AttributeIndexAggregationTypePrimaryShards,
	)
	rmb.RecordElasticsearchIndexOperationsTimeDataPoint(
		now, stats.Primaries.IndexingOperations.IndexTimeInMs, metadata.AttributeOperationIndex, metadata.AttributeIndexAggregationTypePrimaryShards,
	)
	rmb.RecordElasticsearchIndexOperationsTimeDataPoint(
		now, stats.Primaries.IndexingOperations.DeleteTimeInMs, metadata.AttributeOperationDelete, metadata.AttributeIndexAggregationTypePrimaryShards,
	)
	rmb.RecordElasticsearchIndexOperationsTimeDataPoint(
		now, stats.Primaries.GetOperation.TotalTimeInMs, metadata.AttributeOperationGet, metadata.AttributeIndexAggregationTypePrimaryShards,
	)
	rmb.RecordElasticsearchIndexOperationsTimeDataPoint(
		now, stats.Primaries.SearchOperations.ScrollTimeInMs, metadata.AttributeOperationScroll, metadata.AttributeIndexAggregationTypePrimaryShards,
	)
	rmb.RecordElasticsearchIndexOperationsTimeDataPoint(
		now, stats.Primaries.SearchOperations.SuggestTimeInMs, metadata.AttributeOperationSuggest, metadata.AttributeIndexAggregationTypePrimaryShards,
	)
	rmb.RecordElasticsearchIndexOperationsTimeDataPoint(
		now, stats.Primaries.MergeOperations.TotalTimeInMs, metadata.AttributeOperationMerge, metadata.AttributeIndexAggregationTypePrimaryShards,
	)
	rmb.RecordElasticsearchIndexOperationsTimeDataPoint(
		now, stats.Primaries.RefreshOperations.TotalTimeInMs, metadata.AttributeOperationRefresh, metadata.AttributeIndexAggregationTypePrimaryShards,
	)
	rmb.RecordElasticsearchIndexOperationsTimeDataPoint(
		now, stats.Primaries.FlushOperations.TotalTimeInMs, metadata.AttributeOperationFlush, metadata.AttributeIndexAggregationTypePrimaryShards,
	)
	rmb.RecordElasticsearchIndexOperationsTimeDataPoint(
		now, stats.Primaries.WarmerOperations.TotalTimeInMs, metadata.AttributeOperationWarmer, metadata.AttributeIndexAggregationTypePrimaryShards,
	)

	rmb.RecordElasticsearchIndexOperationsMergeSizeDataPoint(
		now, stats.Total.MergeOperations.TotalSizeInBytes, metadata.AttributeIndexAggregationTypeTotal,
	)
	rmb.RecordElasticsearchIndexOperationsMergeDocsCountDataPoint(
		now, stats.Total.MergeOperations.TotalDocs, metadata.AttributeIndexAggregationTypeTotal,
	)

	rmb.RecordElasticsearchIndexShardsSizeDataPoint(
		now, stats.Total.StoreInfo.SizeInBy, metadata.AttributeIndexAggregationTypeTotal,
	)

	rmb.RecordElasticsearchIndexSegmentsCountDataPoint(
		now, stats.Total.SegmentsStats.Count, metadata.AttributeIndexAggregationTypeTotal,
	)
	rmb.RecordElasticsearchIndexSegmentsCountDataPoint(
		now, stats.Primaries.SegmentsStats.Count, metadata.AttributeIndexAggregationTypePrimaryShards,
	)

	rmb.RecordElasticsearchIndexSegmentsSizeDataPoint(
		now, stats.Total.SegmentsStats.MemoryInBy, metadata.AttributeIndexAggregationTypeTotal,
	)
	rmb.RecordElasticsearchIndexSegmentsSizeDataPoint(
		now, stats.Primaries.SegmentsStats.MemoryInBy, metadata.AttributeIndexAggregationTypePrimaryShards,
	)

	rmb.RecordElasticsearchIndexSegmentsMemoryDataPoint(
		now,
		stats.Total.SegmentsStats.DocumentValuesMemoryInBy,
		metadata.AttributeIndexAggregationTypeTotal,
		metadata.AttributeSegmentsMemoryObjectTypeDocValue,
	)
	rmb.RecordElasticsearchIndexSegmentsMemoryDataPoint(
		now,
		stats.Total.SegmentsStats.FixedBitSetMemoryInBy,
		metadata.AttributeIndexAggregationTypeTotal,
		metadata.AttributeSegmentsMemoryObjectTypeFixedBitSet,
	)
	rmb.RecordElasticsearchIndexSegmentsMemoryDataPoint(
		now,
		stats.Total.SegmentsStats.IndexWriterMemoryInBy,
		metadata.AttributeIndexAggregationTypeTotal,
		metadata.AttributeSegmentsMemoryObjectTypeIndexWriter,
	)
	rmb.RecordElasticsearchIndexSegmentsMemoryDataPoint(
		now,
		stats.Total.SegmentsStats.TermsMemoryInBy,
		metadata.AttributeIndexAggregationTypeTotal,
		metadata.AttributeSegmentsMemoryObjectTypeTerm,
	)
	rmb.RecordElasticsearchIndexSegmentsMemoryDataPoint(
		now,
		stats.Primaries.SegmentsStats.DocumentValuesMemoryInBy,
		metadata.AttributeIndexAggregationTypePrimaryShards,
		metadata.AttributeSegmentsMemoryObjectTypeDocValue,
	)
	rmb.RecordElasticsearchIndexSegmentsMemoryDataPoint(
		now,
		stats.Primaries.SegmentsStats.FixedBitSetMemoryInBy,
		metadata.AttributeIndexAggregationTypePrimaryShards,
		metadata.AttributeSegmentsMemoryObjectTypeFixedBitSet,
	)
	rmb.RecordElasticsearchIndexSegmentsMemoryDataPoint(
		now,
		stats.Primaries.SegmentsStats.IndexWriterMemoryInBy,
		metadata.AttributeIndexAggregationTypePrimaryShards,
		metadata.AttributeSegmentsMemoryObjectTypeIndexWriter,
	)
	rmb.RecordElasticsearchIndexSegmentsMemoryDataPoint(
		now,
		stats.Primaries.SegmentsStats.TermsMemoryInBy,
		metadata.AttributeIndexAggregationTypePrimaryShards,
		metadata.AttributeSegmentsMemoryObjectTypeTerm,
	)

	rmb.RecordElasticsearchIndexTranslogOperationsDataPoint(
		now, stats.Total.TranslogStats.Operations, metadata.AttributeIndexAggregationTypeTotal,
	)
	rmb.RecordElasticsearchIndexTranslogSizeDataPoint(
		now, stats.Total.TranslogStats.SizeInBy, metadata.AttributeIndexAggregationTypeTotal,
	)

	rmb.RecordElasticsearchIndexCacheMemoryUsageDataPoint(
		now, stats.Primaries.FieldDataCache.MemorySizeInBy, metadata.AttributeCacheNameFielddata, metadata.AttributeIndexAggregationTypePrimaryShards,
	)
	rmb.RecordElasticsearchIndexCacheMemoryUsageDataPoint(
		now, stats.Total.FieldDataCache.MemorySizeInBy, metadata.AttributeCacheNameFielddata, metadata.AttributeIndexAggregationTypeTotal,
	)
	rmb.RecordElasticsearchIndexCacheMemoryUsageDataPoint(
		now, stats.Total.QueryCache.MemorySizeInBy, metadata.AttributeCacheNameQuery, metadata.AttributeIndexAggregationTypeTotal,
	)
	rmb.RecordElasticsearchIndexCacheMemoryUsageDataPoint(
		now, stats.Primaries.QueryCache.MemorySizeInBy, metadata.AttributeCacheNameQuery, metadata.AttributeIndexAggregationTypePrimaryShards,
	)

	rmb.RecordElasticsearchIndexCacheSizeDataPoint(
		now, stats.Primaries.QueryCache.CacheSize, metadata.AttributeIndexAggregationTypePrimaryShards,
	)
	rmb.RecordElasticsearchIndexCacheSizeDataPoint(
		now, stats.Total.QueryCache.CacheSize, metadata.AttributeIndexAggregationTypeTotal,
	)

	rmb.RecordElasticsearchIndexCacheEvictionsDataPoint(
		now, stats.Primaries.QueryCache.Evictions, metadata.AttributeCacheNameQuery, metadata.AttributeIndexAggregationTypePrimaryShards,
	)
	rmb.RecordElasticsearchIndexCacheEvictionsDataPoint(
		now, stats.Total.QueryCache.Evictions, metadata.AttributeCacheNameQuery, metadata.AttributeIndexAggregationTypeTotal,
	)

	rmb.RecordElasticsearchIndexDocumentsDataPoint(
		now, stats.Primaries.DocumentStats.ActiveCount, metadata.AttributeDocumentStateActive, metadata.AttributeIndexAggregationTypePrimaryShards,
	)
	rmb.RecordElasticsearchIndexDocumentsDataPoint(
		now, stats.Total.DocumentStats.ActiveCount, metadata.AttributeDocumentStateActive, metadata.AttributeIndexAggregationTypeTotal,
	)
}
