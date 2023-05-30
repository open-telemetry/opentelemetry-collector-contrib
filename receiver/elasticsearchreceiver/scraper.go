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
)

const (
	readmeURL = "https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/receiver/elasticsearchreceiver/README.md"
)

var (
	emitNodeVersionAttr = featuregate.GlobalRegistry().MustRegister(
		"receiver.elasticsearch.emitNodeVersionAttr",
		featuregate.StageBeta,
		featuregate.WithRegisterDescription("When enabled, all node metrics will be enriched with the node version resource attribute."),
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

	// Feature gates
	emitNodeVersionAttr bool
}

func newElasticSearchScraper(
	settings receiver.CreateSettings,
	cfg *Config,
) *elasticsearchScraper {
	e := &elasticsearchScraper{
		settings:            settings.TelemetrySettings,
		cfg:                 cfg,
		mb:                  metadata.NewMetricsBuilder(cfg.MetricsBuilderConfig, settings),
		emitNodeVersionAttr: emitNodeVersionAttr.IsEnabled(),
	}

	if !e.emitNodeVersionAttr {
		settings.Logger.Warn(
			fmt.Sprintf("Feature gate %s is not enabled. Please see the README for more information: %s", emitNodeVersionAttr.ID(), readmeURL),
		)
	}

	return e
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

	var nodesInfo *model.Nodes
	if r.emitNodeVersionAttr {
		// Certain node metadata is not available from the /_nodes/stats endpoint. Therefore, we need to get this metadata
		// from the /_nodes endpoint. The metadata may or may not be used depending on feature gates.
		nodesInfo, err = r.client.Nodes(ctx, r.cfg.Nodes)
		if err != nil {
			errs.AddPartial(26, err)
			return
		}
	}

	for id, info := range nodeStats.Nodes {
		r.mb.RecordElasticsearchNodeCacheMemoryUsageDataPoint(now, info.Indices.FieldDataCache.MemorySizeInBy, metadata.AttributeCacheNameFielddata)
		r.mb.RecordElasticsearchNodeCacheMemoryUsageDataPoint(now, info.Indices.QueryCache.MemorySizeInBy, metadata.AttributeCacheNameQuery)

		r.mb.RecordElasticsearchNodeCacheEvictionsDataPoint(now, info.Indices.FieldDataCache.Evictions, metadata.AttributeCacheNameFielddata)
		r.mb.RecordElasticsearchNodeCacheEvictionsDataPoint(now, info.Indices.QueryCache.Evictions, metadata.AttributeCacheNameQuery)

		r.mb.RecordElasticsearchNodeCacheCountDataPoint(now, info.Indices.QueryCache.HitCount, metadata.AttributeQueryCacheCountTypeHit)
		r.mb.RecordElasticsearchNodeCacheCountDataPoint(now, info.Indices.QueryCache.MissCount, metadata.AttributeQueryCacheCountTypeMiss)

		r.mb.RecordElasticsearchNodeCacheSizeDataPoint(now, info.Indices.QueryCache.MemorySizeInBy)

		r.mb.RecordElasticsearchNodeFsDiskAvailableDataPoint(now, info.FS.Total.AvailableBytes)
		r.mb.RecordElasticsearchNodeFsDiskFreeDataPoint(now, info.FS.Total.FreeBytes)
		r.mb.RecordElasticsearchNodeFsDiskTotalDataPoint(now, info.FS.Total.TotalBytes)

		r.mb.RecordElasticsearchNodeDiskIoReadDataPoint(now, info.FS.IOStats.Total.ReadBytes)
		r.mb.RecordElasticsearchNodeDiskIoWriteDataPoint(now, info.FS.IOStats.Total.WriteBytes)

		r.mb.RecordElasticsearchNodeClusterIoDataPoint(now, info.TransportStats.ReceivedBytes, metadata.AttributeDirectionReceived)
		r.mb.RecordElasticsearchNodeClusterIoDataPoint(now, info.TransportStats.SentBytes, metadata.AttributeDirectionSent)

		r.mb.RecordElasticsearchNodeClusterConnectionsDataPoint(now, info.TransportStats.OpenConnections)

		r.mb.RecordElasticsearchNodeHTTPConnectionsDataPoint(now, info.HTTPStats.OpenConnections)

		r.mb.RecordElasticsearchNodeOperationsCurrentDataPoint(now, info.Indices.SearchOperations.QueryCurrent, metadata.AttributeOperationQuery)

		r.mb.RecordElasticsearchNodeOperationsCompletedDataPoint(now, info.Indices.IndexingOperations.IndexTotal, metadata.AttributeOperationIndex)
		r.mb.RecordElasticsearchNodeOperationsCompletedDataPoint(now, info.Indices.IndexingOperations.DeleteTotal, metadata.AttributeOperationDelete)
		r.mb.RecordElasticsearchNodeOperationsCompletedDataPoint(now, info.Indices.GetOperation.Total, metadata.AttributeOperationGet)
		r.mb.RecordElasticsearchNodeOperationsCompletedDataPoint(now, info.Indices.SearchOperations.QueryTotal, metadata.AttributeOperationQuery)
		r.mb.RecordElasticsearchNodeOperationsCompletedDataPoint(now, info.Indices.SearchOperations.FetchTotal, metadata.AttributeOperationFetch)
		r.mb.RecordElasticsearchNodeOperationsCompletedDataPoint(now, info.Indices.SearchOperations.ScrollTotal, metadata.AttributeOperationScroll)
		r.mb.RecordElasticsearchNodeOperationsCompletedDataPoint(now, info.Indices.SearchOperations.SuggestTotal, metadata.AttributeOperationSuggest)
		r.mb.RecordElasticsearchNodeOperationsCompletedDataPoint(now, info.Indices.MergeOperations.Total, metadata.AttributeOperationMerge)
		r.mb.RecordElasticsearchNodeOperationsCompletedDataPoint(now, info.Indices.RefreshOperations.Total, metadata.AttributeOperationRefresh)
		r.mb.RecordElasticsearchNodeOperationsCompletedDataPoint(now, info.Indices.FlushOperations.Total, metadata.AttributeOperationFlush)
		r.mb.RecordElasticsearchNodeOperationsCompletedDataPoint(now, info.Indices.WarmerOperations.Total, metadata.AttributeOperationWarmer)

		r.mb.RecordElasticsearchNodeOperationsTimeDataPoint(now, info.Indices.IndexingOperations.IndexTimeInMs, metadata.AttributeOperationIndex)
		r.mb.RecordElasticsearchNodeOperationsTimeDataPoint(now, info.Indices.IndexingOperations.DeleteTimeInMs, metadata.AttributeOperationDelete)
		r.mb.RecordElasticsearchNodeOperationsTimeDataPoint(now, info.Indices.GetOperation.TotalTimeInMs, metadata.AttributeOperationGet)
		r.mb.RecordElasticsearchNodeOperationsTimeDataPoint(now, info.Indices.SearchOperations.QueryTimeInMs, metadata.AttributeOperationQuery)
		r.mb.RecordElasticsearchNodeOperationsTimeDataPoint(now, info.Indices.SearchOperations.FetchTimeInMs, metadata.AttributeOperationFetch)
		r.mb.RecordElasticsearchNodeOperationsTimeDataPoint(now, info.Indices.SearchOperations.ScrollTimeInMs, metadata.AttributeOperationScroll)
		r.mb.RecordElasticsearchNodeOperationsTimeDataPoint(now, info.Indices.SearchOperations.SuggestTimeInMs, metadata.AttributeOperationSuggest)
		r.mb.RecordElasticsearchNodeOperationsTimeDataPoint(now, info.Indices.MergeOperations.TotalTimeInMs, metadata.AttributeOperationMerge)
		r.mb.RecordElasticsearchNodeOperationsTimeDataPoint(now, info.Indices.RefreshOperations.TotalTimeInMs, metadata.AttributeOperationRefresh)
		r.mb.RecordElasticsearchNodeOperationsTimeDataPoint(now, info.Indices.FlushOperations.TotalTimeInMs, metadata.AttributeOperationFlush)
		r.mb.RecordElasticsearchNodeOperationsTimeDataPoint(now, info.Indices.WarmerOperations.TotalTimeInMs, metadata.AttributeOperationWarmer)

		r.mb.RecordElasticsearchNodeOperationsGetCompletedDataPoint(now, info.Indices.GetOperation.Exists, metadata.AttributeGetResultHit)
		r.mb.RecordElasticsearchNodeOperationsGetCompletedDataPoint(now, info.Indices.GetOperation.Missing, metadata.AttributeGetResultMiss)

		r.mb.RecordElasticsearchNodeOperationsGetTimeDataPoint(now, info.Indices.GetOperation.ExistsTimeInMs, metadata.AttributeGetResultHit)
		r.mb.RecordElasticsearchNodeOperationsGetTimeDataPoint(now, info.Indices.GetOperation.MissingTimeInMs, metadata.AttributeGetResultMiss)

		r.mb.RecordElasticsearchNodeShardsSizeDataPoint(now, info.Indices.StoreInfo.SizeInBy)

		// Elasticsearch version 7.13+ is required to collect `elasticsearch.node.shards.data_set.size`.
		// Reference: https://github.com/elastic/elasticsearch/pull/70625/files#diff-354b5b1f25978b5c638cb707622ae79b42b40aace6f27f3f9d5dd1e31e67b1caR7
		if r.version != nil && r.version.GreaterThanOrEqual(es7_13) {
			r.mb.RecordElasticsearchNodeShardsDataSetSizeDataPoint(now, info.Indices.StoreInfo.DataSetSizeInBy)
		}

		r.mb.RecordElasticsearchNodeShardsReservedSizeDataPoint(now, info.Indices.StoreInfo.ReservedInBy)

		for tpName, tpInfo := range info.ThreadPoolInfo {
			r.mb.RecordElasticsearchNodeThreadPoolThreadsDataPoint(now, tpInfo.ActiveThreads, tpName, metadata.AttributeThreadStateActive)
			r.mb.RecordElasticsearchNodeThreadPoolThreadsDataPoint(now, tpInfo.TotalThreads-tpInfo.ActiveThreads, tpName, metadata.AttributeThreadStateIdle)

			r.mb.RecordElasticsearchNodeThreadPoolTasksQueuedDataPoint(now, tpInfo.QueuedTasks, tpName)

			r.mb.RecordElasticsearchNodeThreadPoolTasksFinishedDataPoint(now, tpInfo.CompletedTasks, tpName, metadata.AttributeTaskStateCompleted)
			r.mb.RecordElasticsearchNodeThreadPoolTasksFinishedDataPoint(now, tpInfo.RejectedTasks, tpName, metadata.AttributeTaskStateRejected)
		}

		for cbName, cbInfo := range info.CircuitBreakerInfo {
			r.mb.RecordElasticsearchBreakerMemoryEstimatedDataPoint(now, cbInfo.EstimatedSizeInBytes, cbName)
			r.mb.RecordElasticsearchBreakerMemoryLimitDataPoint(now, cbInfo.LimitSizeInBytes, cbName)
			r.mb.RecordElasticsearchBreakerTrippedDataPoint(now, cbInfo.Tripped, cbName)
		}

		r.mb.RecordElasticsearchNodeDocumentsDataPoint(now, info.Indices.DocumentStats.ActiveCount, metadata.AttributeDocumentStateActive)
		r.mb.RecordElasticsearchNodeDocumentsDataPoint(now, info.Indices.DocumentStats.DeletedCount, metadata.AttributeDocumentStateDeleted)

		r.mb.RecordElasticsearchNodeOpenFilesDataPoint(now, info.ProcessStats.OpenFileDescriptorsCount)

		r.mb.RecordElasticsearchNodeTranslogOperationsDataPoint(now, info.Indices.TranslogStats.Operations)
		r.mb.RecordElasticsearchNodeTranslogSizeDataPoint(now, info.Indices.TranslogStats.SizeInBy)
		r.mb.RecordElasticsearchNodeTranslogUncommittedSizeDataPoint(now, info.Indices.TranslogStats.UncommittedOperationsInBy)

		r.mb.RecordElasticsearchOsCPUUsageDataPoint(now, info.OS.CPU.Usage)
		r.mb.RecordElasticsearchOsCPULoadAvg1mDataPoint(now, info.OS.CPU.LoadAvg.OneMinute)
		r.mb.RecordElasticsearchOsCPULoadAvg5mDataPoint(now, info.OS.CPU.LoadAvg.FiveMinutes)
		r.mb.RecordElasticsearchOsCPULoadAvg15mDataPoint(now, info.OS.CPU.LoadAvg.FifteenMinutes)

		// Elasticsearch sends this data in percent, but we want to represent it as a number between 0 and 1, so we need to divide.
		// Additionally, if the usage is not known, ES will send '-1'. We do not want to report the metric in this case.
		if info.ProcessStats.CPU.Percent != -1 {
			r.mb.RecordElasticsearchProcessCPUUsageDataPoint(now, float64(info.ProcessStats.CPU.Percent)/100)
		}
		if info.ProcessStats.CPU.TotalInMs != -1 {
			r.mb.RecordElasticsearchProcessCPUTimeDataPoint(now, info.ProcessStats.CPU.TotalInMs)
		}

		r.mb.RecordElasticsearchProcessMemoryVirtualDataPoint(now, info.ProcessStats.Memory.TotalVirtualInBy)

		r.mb.RecordElasticsearchOsMemoryDataPoint(now, info.OS.Memory.UsedInBy, metadata.AttributeMemoryStateUsed)
		r.mb.RecordElasticsearchOsMemoryDataPoint(now, info.OS.Memory.FreeInBy, metadata.AttributeMemoryStateFree)

		r.mb.RecordJvmClassesLoadedDataPoint(now, info.JVMInfo.ClassInfo.CurrentLoadedCount)

		r.mb.RecordJvmGcCollectionsCountDataPoint(now, info.JVMInfo.JVMGCInfo.Collectors.Young.CollectionCount, "young")
		r.mb.RecordJvmGcCollectionsCountDataPoint(now, info.JVMInfo.JVMGCInfo.Collectors.Old.CollectionCount, "old")

		r.mb.RecordJvmGcCollectionsElapsedDataPoint(now, info.JVMInfo.JVMGCInfo.Collectors.Young.CollectionTimeInMillis, "young")
		r.mb.RecordJvmGcCollectionsElapsedDataPoint(now, info.JVMInfo.JVMGCInfo.Collectors.Old.CollectionTimeInMillis, "old")

		r.mb.RecordJvmMemoryHeapMaxDataPoint(now, info.JVMInfo.JVMMemoryInfo.MaxHeapInBy)
		r.mb.RecordJvmMemoryHeapUsedDataPoint(now, info.JVMInfo.JVMMemoryInfo.HeapUsedInBy)
		r.mb.RecordJvmMemoryHeapCommittedDataPoint(now, info.JVMInfo.JVMMemoryInfo.HeapCommittedInBy)
		// Elasticsearch sends this data in percent, but we want to represent it as a number between 0 and 1, so we need to divide.
		r.mb.RecordJvmMemoryHeapUtilizationDataPoint(now, float64(info.JVMInfo.JVMMemoryInfo.HeapUsedPercent)/100)

		r.mb.RecordJvmMemoryNonheapUsedDataPoint(now, info.JVMInfo.JVMMemoryInfo.NonHeapUsedInBy)
		r.mb.RecordJvmMemoryNonheapCommittedDataPoint(now, info.JVMInfo.JVMMemoryInfo.NonHeapComittedInBy)

		r.mb.RecordJvmMemoryPoolUsedDataPoint(now, info.JVMInfo.JVMMemoryInfo.MemoryPools.Young.MemUsedBy, "young")
		r.mb.RecordJvmMemoryPoolUsedDataPoint(now, info.JVMInfo.JVMMemoryInfo.MemoryPools.Survivor.MemUsedBy, "survivor")
		r.mb.RecordJvmMemoryPoolUsedDataPoint(now, info.JVMInfo.JVMMemoryInfo.MemoryPools.Old.MemUsedBy, "old")

		r.mb.RecordJvmMemoryPoolMaxDataPoint(now, info.JVMInfo.JVMMemoryInfo.MemoryPools.Young.MemMaxBy, "young")
		r.mb.RecordJvmMemoryPoolMaxDataPoint(now, info.JVMInfo.JVMMemoryInfo.MemoryPools.Survivor.MemMaxBy, "survivor")
		r.mb.RecordJvmMemoryPoolMaxDataPoint(now, info.JVMInfo.JVMMemoryInfo.MemoryPools.Old.MemMaxBy, "old")

		r.mb.RecordJvmThreadsCountDataPoint(now, info.JVMInfo.JVMThreadInfo.Count)

		// Elasticsearch version 7.10+ is required to collect `elasticsearch.indexing_pressure.memory.limit`.
		// Reference: https://github.com/elastic/elasticsearch/pull/60342/files#diff-13864344bab3afc267797d67b2746e2939a3fd8af7611ac9fbda376323e2f5eaR37
		if r.version != nil && r.version.GreaterThanOrEqual(es7_10) {
			r.mb.RecordElasticsearchIndexingPressureMemoryLimitDataPoint(now, info.IndexingPressure.Memory.LimitInBy)
		}

		r.mb.RecordElasticsearchMemoryIndexingPressureDataPoint(now, info.IndexingPressure.Memory.Current.PrimaryInBy, metadata.AttributeIndexingPressureStagePrimary)
		r.mb.RecordElasticsearchMemoryIndexingPressureDataPoint(now, info.IndexingPressure.Memory.Current.CoordinatingInBy, metadata.AttributeIndexingPressureStageCoordinating)
		r.mb.RecordElasticsearchMemoryIndexingPressureDataPoint(now, info.IndexingPressure.Memory.Current.ReplicaInBy, metadata.AttributeIndexingPressureStageReplica)
		r.mb.RecordElasticsearchIndexingPressureMemoryTotalPrimaryRejectionsDataPoint(now, info.IndexingPressure.Memory.Total.PrimaryRejections)
		r.mb.RecordElasticsearchIndexingPressureMemoryTotalReplicaRejectionsDataPoint(now, info.IndexingPressure.Memory.Total.ReplicaRejections)

		r.mb.RecordElasticsearchClusterStateQueueDataPoint(now, info.Discovery.ClusterStateQueue.Committed, metadata.AttributeClusterStateQueueStateCommitted)
		r.mb.RecordElasticsearchClusterStateQueueDataPoint(now, info.Discovery.ClusterStateQueue.Committed, metadata.AttributeClusterStateQueueStatePending)

		r.mb.RecordElasticsearchClusterPublishedStatesFullDataPoint(now, info.Discovery.PublishedClusterStates.FullStates)
		r.mb.RecordElasticsearchClusterPublishedStatesDifferencesDataPoint(now, info.Discovery.PublishedClusterStates.CompatibleDiffs, metadata.AttributeClusterPublishedDifferenceStateCompatible)
		r.mb.RecordElasticsearchClusterPublishedStatesDifferencesDataPoint(now, info.Discovery.PublishedClusterStates.IncompatibleDiffs, metadata.AttributeClusterPublishedDifferenceStateIncompatible)

		for cusState, csuInfo := range info.Discovery.ClusterStateUpdate {
			r.mb.RecordElasticsearchClusterStateUpdateCountDataPoint(now, csuInfo.Count, cusState)
			r.mb.RecordElasticsearchClusterStateUpdateTimeDataPoint(now, csuInfo.ComputationTimeMillis, cusState, metadata.AttributeClusterStateUpdateTypeComputation)
			r.mb.RecordElasticsearchClusterStateUpdateTimeDataPoint(now, csuInfo.NotificationTimeMillis, cusState, metadata.AttributeClusterStateUpdateTypeNotification)
			if cusState == "unchanged" {
				// the node_linux.json payload response for "elasticsearch.cluster.state_update.time" with attributes "unchanged" has 2 attributes "computation_time_millis" and "notification_time_millis".
				// All other metrics for "unchanged" should be skipped to prevent 0 emitted metrics"
				// https://github.com/elastic/elasticsearch/pull/76771/files#diff-8bbfc581d91f9440e53098ea7d7864aeaeac1fc83a714133e4aafe38eba8ed90R2098
				continue
			}
			r.mb.RecordElasticsearchClusterStateUpdateTimeDataPoint(now, csuInfo.ContextConstructionTimeMillis, cusState, metadata.AttributeClusterStateUpdateTypeContextConstruction)
			r.mb.RecordElasticsearchClusterStateUpdateTimeDataPoint(now, csuInfo.CommitTimeMillis, cusState, metadata.AttributeClusterStateUpdateTypeCommit)
			r.mb.RecordElasticsearchClusterStateUpdateTimeDataPoint(now, csuInfo.CompletionTimeMillis, cusState, metadata.AttributeClusterStateUpdateTypeCompletion)
			r.mb.RecordElasticsearchClusterStateUpdateTimeDataPoint(now, csuInfo.MasterApplyTimeMillis, cusState, metadata.AttributeClusterStateUpdateTypeMasterApply)
		}

		r.mb.RecordElasticsearchNodeIngestDocumentsDataPoint(now, info.Ingest.Total.Count)
		r.mb.RecordElasticsearchNodeIngestDocumentsCurrentDataPoint(now, info.Ingest.Total.Current)
		r.mb.RecordElasticsearchNodeIngestOperationsFailedDataPoint(now, info.Ingest.Total.Failed)

		for ipName, ipInfo := range info.Ingest.Pipelines {
			r.mb.RecordElasticsearchNodePipelineIngestDocumentsPreprocessedDataPoint(now, ipInfo.Count, ipName)
			r.mb.RecordElasticsearchNodePipelineIngestOperationsFailedDataPoint(now, ipInfo.Failed, ipName)
			r.mb.RecordElasticsearchNodePipelineIngestDocumentsCurrentDataPoint(now, ipInfo.Current, ipName)
		}

		r.mb.RecordElasticsearchNodeScriptCacheEvictionsDataPoint(now, info.Script.CacheEvictions)
		r.mb.RecordElasticsearchNodeScriptCompilationsDataPoint(now, info.Script.Compilations)
		r.mb.RecordElasticsearchNodeScriptCompilationLimitTriggeredDataPoint(now, info.Script.CompilationLimitTriggered)

		r.mb.RecordElasticsearchNodeSegmentsMemoryDataPoint(
			now, info.Indices.SegmentsStats.DocumentValuesMemoryInBy, metadata.AttributeSegmentsMemoryObjectTypeDocValue,
		)
		r.mb.RecordElasticsearchNodeSegmentsMemoryDataPoint(
			now, info.Indices.SegmentsStats.FixedBitSetMemoryInBy, metadata.AttributeSegmentsMemoryObjectTypeFixedBitSet,
		)
		r.mb.RecordElasticsearchNodeSegmentsMemoryDataPoint(
			now, info.Indices.SegmentsStats.IndexWriterMemoryInBy, metadata.AttributeSegmentsMemoryObjectTypeIndexWriter,
		)
		r.mb.RecordElasticsearchNodeSegmentsMemoryDataPoint(
			now, info.Indices.SegmentsStats.TermsMemoryInBy, metadata.AttributeSegmentsMemoryObjectTypeTerm,
		)

		// Define nodeMetadata slice to store all metadata. New metadata can be easily introduced by appending to the slice.
		nodeMetadata := []metadata.ResourceMetricsOption{
			metadata.WithElasticsearchClusterName(nodeStats.ClusterName),
			metadata.WithElasticsearchNodeName(info.Name),
		}

		if r.emitNodeVersionAttr {
			if node, ok := nodesInfo.Nodes[id]; ok {
				nodeMetadata = append(nodeMetadata, metadata.WithElasticsearchNodeVersion(node.Version))
			}
		}

		r.mb.EmitForResource(nodeMetadata...)
	}
}

func (r *elasticsearchScraper) scrapeClusterMetrics(ctx context.Context, now pcommon.Timestamp, errs *scrapererror.ScrapeErrors) {
	if r.cfg.SkipClusterMetrics {
		return
	}

	r.scrapeClusterHealthMetrics(ctx, now, errs)
	r.scrapeClusterStatsMetrics(ctx, now, errs)

	r.mb.EmitForResource(metadata.WithElasticsearchClusterName(r.clusterName))
}

func (r *elasticsearchScraper) scrapeClusterStatsMetrics(ctx context.Context, now pcommon.Timestamp, errs *scrapererror.ScrapeErrors) {
	if len(r.cfg.Nodes) == 0 {
		return
	}

	clusterStats, err := r.client.ClusterStats(ctx, r.cfg.Nodes)
	if err != nil {
		errs.AddPartial(3, err)
		return
	}

	r.mb.RecordJvmMemoryHeapUsedDataPoint(now, clusterStats.NodesStats.JVMInfo.JVMMemoryInfo.HeapUsedInBy)

	r.mb.RecordElasticsearchClusterIndicesCacheEvictionsDataPoint(
		now, clusterStats.IndicesStats.FieldDataCache.Evictions, metadata.AttributeCacheNameFielddata,
	)
	r.mb.RecordElasticsearchClusterIndicesCacheEvictionsDataPoint(
		now, clusterStats.IndicesStats.QueryCache.Evictions, metadata.AttributeCacheNameQuery,
	)
}

func (r *elasticsearchScraper) scrapeClusterHealthMetrics(ctx context.Context, now pcommon.Timestamp, errs *scrapererror.ScrapeErrors) {
	clusterHealth, err := r.client.ClusterHealth(ctx)
	if err != nil {
		errs.AddPartial(4, err)
		return
	}

	r.mb.RecordElasticsearchClusterNodesDataPoint(now, clusterHealth.NodeCount)

	r.mb.RecordElasticsearchClusterDataNodesDataPoint(now, clusterHealth.DataNodeCount)

	r.mb.RecordElasticsearchClusterShardsDataPoint(now, clusterHealth.ActiveShards, metadata.AttributeShardStateActive)
	r.mb.RecordElasticsearchClusterShardsDataPoint(now, clusterHealth.InitializingShards, metadata.AttributeShardStateInitializing)
	r.mb.RecordElasticsearchClusterShardsDataPoint(now, clusterHealth.RelocatingShards, metadata.AttributeShardStateRelocating)
	r.mb.RecordElasticsearchClusterShardsDataPoint(now, clusterHealth.UnassignedShards, metadata.AttributeShardStateUnassigned)
	r.mb.RecordElasticsearchClusterShardsDataPoint(now, clusterHealth.ActivePrimaryShards, metadata.AttributeShardStateActivePrimary)
	r.mb.RecordElasticsearchClusterShardsDataPoint(now, clusterHealth.DelayedUnassignedShards, metadata.AttributeShardStateUnassignedDelayed)

	r.mb.RecordElasticsearchClusterPendingTasksDataPoint(now, clusterHealth.PendingTasksCount)
	r.mb.RecordElasticsearchClusterInFlightFetchDataPoint(now, clusterHealth.InFlightFetchCount)

	switch clusterHealth.Status {
	case "green":
		r.mb.RecordElasticsearchClusterHealthDataPoint(now, 1, metadata.AttributeHealthStatusGreen)
		r.mb.RecordElasticsearchClusterHealthDataPoint(now, 0, metadata.AttributeHealthStatusYellow)
		r.mb.RecordElasticsearchClusterHealthDataPoint(now, 0, metadata.AttributeHealthStatusRed)
	case "yellow":
		r.mb.RecordElasticsearchClusterHealthDataPoint(now, 0, metadata.AttributeHealthStatusGreen)
		r.mb.RecordElasticsearchClusterHealthDataPoint(now, 1, metadata.AttributeHealthStatusYellow)
		r.mb.RecordElasticsearchClusterHealthDataPoint(now, 0, metadata.AttributeHealthStatusRed)
	case "red":
		r.mb.RecordElasticsearchClusterHealthDataPoint(now, 0, metadata.AttributeHealthStatusGreen)
		r.mb.RecordElasticsearchClusterHealthDataPoint(now, 0, metadata.AttributeHealthStatusYellow)
		r.mb.RecordElasticsearchClusterHealthDataPoint(now, 1, metadata.AttributeHealthStatusRed)
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
	r.mb.RecordElasticsearchIndexOperationsCompletedDataPoint(
		now, stats.Total.SearchOperations.FetchTotal, metadata.AttributeOperationFetch, metadata.AttributeIndexAggregationTypeTotal,
	)
	r.mb.RecordElasticsearchIndexOperationsCompletedDataPoint(
		now, stats.Total.SearchOperations.QueryTotal, metadata.AttributeOperationQuery, metadata.AttributeIndexAggregationTypeTotal,
	)
	r.mb.RecordElasticsearchIndexOperationsCompletedDataPoint(
		now, stats.Total.IndexingOperations.IndexTotal, metadata.AttributeOperationIndex, metadata.AttributeIndexAggregationTypeTotal,
	)
	r.mb.RecordElasticsearchIndexOperationsCompletedDataPoint(
		now, stats.Total.IndexingOperations.DeleteTotal, metadata.AttributeOperationDelete, metadata.AttributeIndexAggregationTypeTotal,
	)
	r.mb.RecordElasticsearchIndexOperationsCompletedDataPoint(
		now, stats.Total.GetOperation.Total, metadata.AttributeOperationGet, metadata.AttributeIndexAggregationTypeTotal,
	)
	r.mb.RecordElasticsearchIndexOperationsCompletedDataPoint(
		now, stats.Total.SearchOperations.ScrollTotal, metadata.AttributeOperationScroll, metadata.AttributeIndexAggregationTypeTotal,
	)
	r.mb.RecordElasticsearchIndexOperationsCompletedDataPoint(
		now, stats.Total.SearchOperations.SuggestTotal, metadata.AttributeOperationSuggest, metadata.AttributeIndexAggregationTypeTotal,
	)
	r.mb.RecordElasticsearchIndexOperationsCompletedDataPoint(
		now, stats.Total.MergeOperations.Total, metadata.AttributeOperationMerge, metadata.AttributeIndexAggregationTypeTotal,
	)
	r.mb.RecordElasticsearchIndexOperationsCompletedDataPoint(
		now, stats.Total.RefreshOperations.Total, metadata.AttributeOperationRefresh, metadata.AttributeIndexAggregationTypeTotal,
	)
	r.mb.RecordElasticsearchIndexOperationsCompletedDataPoint(
		now, stats.Total.FlushOperations.Total, metadata.AttributeOperationFlush, metadata.AttributeIndexAggregationTypeTotal,
	)
	r.mb.RecordElasticsearchIndexOperationsCompletedDataPoint(
		now, stats.Total.WarmerOperations.Total, metadata.AttributeOperationWarmer, metadata.AttributeIndexAggregationTypeTotal,
	)

	r.mb.RecordElasticsearchIndexOperationsCompletedDataPoint(
		now, stats.Primaries.SearchOperations.FetchTotal, metadata.AttributeOperationFetch, metadata.AttributeIndexAggregationTypePrimaryShards,
	)
	r.mb.RecordElasticsearchIndexOperationsCompletedDataPoint(
		now, stats.Primaries.SearchOperations.QueryTotal, metadata.AttributeOperationQuery, metadata.AttributeIndexAggregationTypePrimaryShards,
	)
	r.mb.RecordElasticsearchIndexOperationsCompletedDataPoint(
		now, stats.Primaries.IndexingOperations.IndexTotal, metadata.AttributeOperationIndex, metadata.AttributeIndexAggregationTypePrimaryShards,
	)
	r.mb.RecordElasticsearchIndexOperationsCompletedDataPoint(
		now, stats.Primaries.IndexingOperations.DeleteTotal, metadata.AttributeOperationDelete, metadata.AttributeIndexAggregationTypePrimaryShards,
	)
	r.mb.RecordElasticsearchIndexOperationsCompletedDataPoint(
		now, stats.Primaries.GetOperation.Total, metadata.AttributeOperationGet, metadata.AttributeIndexAggregationTypePrimaryShards,
	)
	r.mb.RecordElasticsearchIndexOperationsCompletedDataPoint(
		now, stats.Primaries.SearchOperations.ScrollTotal, metadata.AttributeOperationScroll, metadata.AttributeIndexAggregationTypePrimaryShards,
	)
	r.mb.RecordElasticsearchIndexOperationsCompletedDataPoint(
		now, stats.Primaries.SearchOperations.SuggestTotal, metadata.AttributeOperationSuggest, metadata.AttributeIndexAggregationTypePrimaryShards,
	)
	r.mb.RecordElasticsearchIndexOperationsCompletedDataPoint(
		now, stats.Primaries.MergeOperations.Total, metadata.AttributeOperationMerge, metadata.AttributeIndexAggregationTypePrimaryShards,
	)
	r.mb.RecordElasticsearchIndexOperationsCompletedDataPoint(
		now, stats.Primaries.RefreshOperations.Total, metadata.AttributeOperationRefresh, metadata.AttributeIndexAggregationTypePrimaryShards,
	)
	r.mb.RecordElasticsearchIndexOperationsCompletedDataPoint(
		now, stats.Primaries.FlushOperations.Total, metadata.AttributeOperationFlush, metadata.AttributeIndexAggregationTypePrimaryShards,
	)
	r.mb.RecordElasticsearchIndexOperationsCompletedDataPoint(
		now, stats.Primaries.WarmerOperations.Total, metadata.AttributeOperationWarmer, metadata.AttributeIndexAggregationTypePrimaryShards,
	)

	r.mb.RecordElasticsearchIndexOperationsTimeDataPoint(
		now, stats.Total.SearchOperations.FetchTimeInMs, metadata.AttributeOperationFetch, metadata.AttributeIndexAggregationTypeTotal,
	)
	r.mb.RecordElasticsearchIndexOperationsTimeDataPoint(
		now, stats.Total.SearchOperations.QueryTimeInMs, metadata.AttributeOperationQuery, metadata.AttributeIndexAggregationTypeTotal,
	)
	r.mb.RecordElasticsearchIndexOperationsTimeDataPoint(
		now, stats.Total.IndexingOperations.IndexTimeInMs, metadata.AttributeOperationIndex, metadata.AttributeIndexAggregationTypeTotal,
	)
	r.mb.RecordElasticsearchIndexOperationsTimeDataPoint(
		now, stats.Total.IndexingOperations.DeleteTimeInMs, metadata.AttributeOperationDelete, metadata.AttributeIndexAggregationTypeTotal,
	)
	r.mb.RecordElasticsearchIndexOperationsTimeDataPoint(
		now, stats.Total.GetOperation.TotalTimeInMs, metadata.AttributeOperationGet, metadata.AttributeIndexAggregationTypeTotal,
	)
	r.mb.RecordElasticsearchIndexOperationsTimeDataPoint(
		now, stats.Total.SearchOperations.ScrollTimeInMs, metadata.AttributeOperationScroll, metadata.AttributeIndexAggregationTypeTotal,
	)
	r.mb.RecordElasticsearchIndexOperationsTimeDataPoint(
		now, stats.Total.SearchOperations.SuggestTimeInMs, metadata.AttributeOperationSuggest, metadata.AttributeIndexAggregationTypeTotal,
	)
	r.mb.RecordElasticsearchIndexOperationsTimeDataPoint(
		now, stats.Total.MergeOperations.TotalTimeInMs, metadata.AttributeOperationMerge, metadata.AttributeIndexAggregationTypeTotal,
	)
	r.mb.RecordElasticsearchIndexOperationsTimeDataPoint(
		now, stats.Total.RefreshOperations.TotalTimeInMs, metadata.AttributeOperationRefresh, metadata.AttributeIndexAggregationTypeTotal,
	)
	r.mb.RecordElasticsearchIndexOperationsTimeDataPoint(
		now, stats.Total.FlushOperations.TotalTimeInMs, metadata.AttributeOperationFlush, metadata.AttributeIndexAggregationTypeTotal,
	)
	r.mb.RecordElasticsearchIndexOperationsTimeDataPoint(
		now, stats.Total.WarmerOperations.TotalTimeInMs, metadata.AttributeOperationWarmer, metadata.AttributeIndexAggregationTypeTotal,
	)

	r.mb.RecordElasticsearchIndexOperationsTimeDataPoint(
		now, stats.Primaries.SearchOperations.FetchTimeInMs, metadata.AttributeOperationFetch, metadata.AttributeIndexAggregationTypePrimaryShards,
	)
	r.mb.RecordElasticsearchIndexOperationsTimeDataPoint(
		now, stats.Primaries.SearchOperations.QueryTimeInMs, metadata.AttributeOperationQuery, metadata.AttributeIndexAggregationTypePrimaryShards,
	)
	r.mb.RecordElasticsearchIndexOperationsTimeDataPoint(
		now, stats.Primaries.IndexingOperations.IndexTimeInMs, metadata.AttributeOperationIndex, metadata.AttributeIndexAggregationTypePrimaryShards,
	)
	r.mb.RecordElasticsearchIndexOperationsTimeDataPoint(
		now, stats.Primaries.IndexingOperations.DeleteTimeInMs, metadata.AttributeOperationDelete, metadata.AttributeIndexAggregationTypePrimaryShards,
	)
	r.mb.RecordElasticsearchIndexOperationsTimeDataPoint(
		now, stats.Primaries.GetOperation.TotalTimeInMs, metadata.AttributeOperationGet, metadata.AttributeIndexAggregationTypePrimaryShards,
	)
	r.mb.RecordElasticsearchIndexOperationsTimeDataPoint(
		now, stats.Primaries.SearchOperations.ScrollTimeInMs, metadata.AttributeOperationScroll, metadata.AttributeIndexAggregationTypePrimaryShards,
	)
	r.mb.RecordElasticsearchIndexOperationsTimeDataPoint(
		now, stats.Primaries.SearchOperations.SuggestTimeInMs, metadata.AttributeOperationSuggest, metadata.AttributeIndexAggregationTypePrimaryShards,
	)
	r.mb.RecordElasticsearchIndexOperationsTimeDataPoint(
		now, stats.Primaries.MergeOperations.TotalTimeInMs, metadata.AttributeOperationMerge, metadata.AttributeIndexAggregationTypePrimaryShards,
	)
	r.mb.RecordElasticsearchIndexOperationsTimeDataPoint(
		now, stats.Primaries.RefreshOperations.TotalTimeInMs, metadata.AttributeOperationRefresh, metadata.AttributeIndexAggregationTypePrimaryShards,
	)
	r.mb.RecordElasticsearchIndexOperationsTimeDataPoint(
		now, stats.Primaries.FlushOperations.TotalTimeInMs, metadata.AttributeOperationFlush, metadata.AttributeIndexAggregationTypePrimaryShards,
	)
	r.mb.RecordElasticsearchIndexOperationsTimeDataPoint(
		now, stats.Primaries.WarmerOperations.TotalTimeInMs, metadata.AttributeOperationWarmer, metadata.AttributeIndexAggregationTypePrimaryShards,
	)

	r.mb.RecordElasticsearchIndexOperationsMergeSizeDataPoint(
		now, stats.Total.MergeOperations.TotalSizeInBytes, metadata.AttributeIndexAggregationTypeTotal,
	)
	r.mb.RecordElasticsearchIndexOperationsMergeDocsCountDataPoint(
		now, stats.Total.MergeOperations.TotalDocs, metadata.AttributeIndexAggregationTypeTotal,
	)

	r.mb.RecordElasticsearchIndexShardsSizeDataPoint(
		now, stats.Total.StoreInfo.SizeInBy, metadata.AttributeIndexAggregationTypeTotal,
	)

	r.mb.RecordElasticsearchIndexSegmentsCountDataPoint(
		now, stats.Total.SegmentsStats.Count, metadata.AttributeIndexAggregationTypeTotal,
	)
	r.mb.RecordElasticsearchIndexSegmentsCountDataPoint(
		now, stats.Primaries.SegmentsStats.Count, metadata.AttributeIndexAggregationTypePrimaryShards,
	)

	r.mb.RecordElasticsearchIndexSegmentsSizeDataPoint(
		now, stats.Total.SegmentsStats.MemoryInBy, metadata.AttributeIndexAggregationTypeTotal,
	)
	r.mb.RecordElasticsearchIndexSegmentsSizeDataPoint(
		now, stats.Primaries.SegmentsStats.MemoryInBy, metadata.AttributeIndexAggregationTypePrimaryShards,
	)

	r.mb.RecordElasticsearchIndexSegmentsMemoryDataPoint(
		now,
		stats.Total.SegmentsStats.DocumentValuesMemoryInBy,
		metadata.AttributeIndexAggregationTypeTotal,
		metadata.AttributeSegmentsMemoryObjectTypeDocValue,
	)
	r.mb.RecordElasticsearchIndexSegmentsMemoryDataPoint(
		now,
		stats.Total.SegmentsStats.FixedBitSetMemoryInBy,
		metadata.AttributeIndexAggregationTypeTotal,
		metadata.AttributeSegmentsMemoryObjectTypeFixedBitSet,
	)
	r.mb.RecordElasticsearchIndexSegmentsMemoryDataPoint(
		now,
		stats.Total.SegmentsStats.IndexWriterMemoryInBy,
		metadata.AttributeIndexAggregationTypeTotal,
		metadata.AttributeSegmentsMemoryObjectTypeIndexWriter,
	)
	r.mb.RecordElasticsearchIndexSegmentsMemoryDataPoint(
		now,
		stats.Total.SegmentsStats.TermsMemoryInBy,
		metadata.AttributeIndexAggregationTypeTotal,
		metadata.AttributeSegmentsMemoryObjectTypeTerm,
	)
	r.mb.RecordElasticsearchIndexSegmentsMemoryDataPoint(
		now,
		stats.Primaries.SegmentsStats.DocumentValuesMemoryInBy,
		metadata.AttributeIndexAggregationTypePrimaryShards,
		metadata.AttributeSegmentsMemoryObjectTypeDocValue,
	)
	r.mb.RecordElasticsearchIndexSegmentsMemoryDataPoint(
		now,
		stats.Primaries.SegmentsStats.FixedBitSetMemoryInBy,
		metadata.AttributeIndexAggregationTypePrimaryShards,
		metadata.AttributeSegmentsMemoryObjectTypeFixedBitSet,
	)
	r.mb.RecordElasticsearchIndexSegmentsMemoryDataPoint(
		now,
		stats.Primaries.SegmentsStats.IndexWriterMemoryInBy,
		metadata.AttributeIndexAggregationTypePrimaryShards,
		metadata.AttributeSegmentsMemoryObjectTypeIndexWriter,
	)
	r.mb.RecordElasticsearchIndexSegmentsMemoryDataPoint(
		now,
		stats.Primaries.SegmentsStats.TermsMemoryInBy,
		metadata.AttributeIndexAggregationTypePrimaryShards,
		metadata.AttributeSegmentsMemoryObjectTypeTerm,
	)

	r.mb.RecordElasticsearchIndexTranslogOperationsDataPoint(
		now, stats.Total.TranslogStats.Operations, metadata.AttributeIndexAggregationTypeTotal,
	)
	r.mb.RecordElasticsearchIndexTranslogSizeDataPoint(
		now, stats.Total.TranslogStats.SizeInBy, metadata.AttributeIndexAggregationTypeTotal,
	)

	r.mb.RecordElasticsearchIndexCacheMemoryUsageDataPoint(
		now, stats.Primaries.FieldDataCache.MemorySizeInBy, metadata.AttributeCacheNameFielddata, metadata.AttributeIndexAggregationTypePrimaryShards,
	)
	r.mb.RecordElasticsearchIndexCacheMemoryUsageDataPoint(
		now, stats.Total.FieldDataCache.MemorySizeInBy, metadata.AttributeCacheNameFielddata, metadata.AttributeIndexAggregationTypeTotal,
	)
	r.mb.RecordElasticsearchIndexCacheMemoryUsageDataPoint(
		now, stats.Total.QueryCache.MemorySizeInBy, metadata.AttributeCacheNameQuery, metadata.AttributeIndexAggregationTypeTotal,
	)
	r.mb.RecordElasticsearchIndexCacheMemoryUsageDataPoint(
		now, stats.Primaries.QueryCache.MemorySizeInBy, metadata.AttributeCacheNameQuery, metadata.AttributeIndexAggregationTypePrimaryShards,
	)

	r.mb.RecordElasticsearchIndexCacheSizeDataPoint(
		now, stats.Primaries.QueryCache.CacheSize, metadata.AttributeIndexAggregationTypePrimaryShards,
	)
	r.mb.RecordElasticsearchIndexCacheSizeDataPoint(
		now, stats.Total.QueryCache.CacheSize, metadata.AttributeIndexAggregationTypeTotal,
	)

	r.mb.RecordElasticsearchIndexCacheEvictionsDataPoint(
		now, stats.Primaries.QueryCache.Evictions, metadata.AttributeCacheNameQuery, metadata.AttributeIndexAggregationTypePrimaryShards,
	)
	r.mb.RecordElasticsearchIndexCacheEvictionsDataPoint(
		now, stats.Total.QueryCache.Evictions, metadata.AttributeCacheNameQuery, metadata.AttributeIndexAggregationTypeTotal,
	)

	r.mb.RecordElasticsearchIndexDocumentsDataPoint(
		now, stats.Primaries.DocumentStats.ActiveCount, metadata.AttributeDocumentStateActive, metadata.AttributeIndexAggregationTypePrimaryShards,
	)
	r.mb.RecordElasticsearchIndexDocumentsDataPoint(
		now, stats.Total.DocumentStats.ActiveCount, metadata.AttributeDocumentStateActive, metadata.AttributeIndexAggregationTypeTotal,
	)

	r.mb.EmitForResource(metadata.WithElasticsearchIndexName(name), metadata.WithElasticsearchClusterName(r.clusterName))
}
