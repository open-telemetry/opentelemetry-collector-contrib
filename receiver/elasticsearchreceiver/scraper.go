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

package elasticsearchreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/elasticsearchreceiver"

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/hashicorp/go-version"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/scrapererror"
	"go.opentelemetry.io/collector/service/featuregate"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/elasticsearchreceiver/internal/metadata"
)

const (
	emitMetricsWithDirectionAttributeFeatureGateID    = "receiver.elasticsearchreceiver.emitMetricsWithDirectionAttribute"
	emitMetricsWithoutDirectionAttributeFeatureGateID = "receiver.elasticsearchreceiver.emitMetricsWithoutDirectionAttribute"
)

var (
	emitMetricsWithDirectionAttributeFeatureGate = featuregate.Gate{
		ID:      emitMetricsWithDirectionAttributeFeatureGateID,
		Enabled: true,
		Description: "Some elasticsearch metrics reported are transitioning from being reported with a direction " +
			"attribute to being reported with the direction included in the metric name to adhere to the " +
			"OpenTelemetry specification. This feature gate controls emitting the old metrics with the direction " +
			"attribute. For more details, see: " +
			"https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/receiver/elasticsearchreceiver/README.md#feature-gate-configurations",
	}

	emitMetricsWithoutDirectionAttributeFeatureGate = featuregate.Gate{
		ID:      emitMetricsWithoutDirectionAttributeFeatureGateID,
		Enabled: false,
		Description: "Some elasticsearch metrics reported are transitioning from being reported with a direction " +
			"attribute to being reported with the direction included in the metric name to adhere to the " +
			"OpenTelemetry specification. This feature gate controls emitting the new metrics without the direction " +
			"attribute. For more details, see: " +
			"https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/receiver/elasticsearchreceiver/README.md#feature-gate-configurations",
	}
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

func init() {
	featuregate.GetRegistry().MustRegister(emitMetricsWithDirectionAttributeFeatureGate)
	featuregate.GetRegistry().MustRegister(emitMetricsWithoutDirectionAttributeFeatureGate)
}

var errUnknownClusterStatus = errors.New("unknown cluster status")

type elasticsearchScraper struct {
	client                               elasticsearchClient
	settings                             component.TelemetrySettings
	cfg                                  *Config
	mb                                   *metadata.MetricsBuilder
	version                              *version.Version
	emitMetricsWithDirectionAttribute    bool
	emitMetricsWithoutDirectionAttribute bool
}

func newElasticSearchScraper(
	settings component.ReceiverCreateSettings,
	cfg *Config,
) *elasticsearchScraper {
	return &elasticsearchScraper{
		settings:                             settings.TelemetrySettings,
		cfg:                                  cfg,
		mb:                                   metadata.NewMetricsBuilder(cfg.Metrics, settings.BuildInfo),
		emitMetricsWithDirectionAttribute:    featuregate.GetRegistry().IsEnabled(emitMetricsWithDirectionAttributeFeatureGateID),
		emitMetricsWithoutDirectionAttribute: featuregate.GetRegistry().IsEnabled(emitMetricsWithoutDirectionAttributeFeatureGateID),
	}
}

func (r *elasticsearchScraper) start(_ context.Context, host component.Host) (err error) {
	r.client, err = newElasticsearchClient(r.settings, *r.cfg, host)
	return
}

func (r *elasticsearchScraper) scrape(ctx context.Context) (pmetric.Metrics, error) {
	errs := &scrapererror.ScrapeErrors{}

	now := pcommon.NewTimestampFromTime(time.Now())

	r.getVersion(ctx, errs)
	r.scrapeNodeMetrics(ctx, now, errs)
	r.scrapeClusterMetrics(ctx, now, errs)

	return r.mb.Emit(), errs.Combine()
}

// scrapeVersion gets and assigns the elasticsearch version number
func (r *elasticsearchScraper) getVersion(ctx context.Context, errs *scrapererror.ScrapeErrors) {
	versionResponse, err := r.client.Version(ctx)
	if err != nil {
		errs.AddPartial(2, err)
		return
	}

	esVersion, err := version.NewVersion(versionResponse.Version.Number)
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

	for _, info := range nodeStats.Nodes {
		r.mb.RecordElasticsearchNodeCacheMemoryUsageDataPoint(now, info.Indices.FieldDataCache.MemorySizeInBy, metadata.AttributeCacheNameFielddata)
		r.mb.RecordElasticsearchNodeCacheMemoryUsageDataPoint(now, info.Indices.QueryCache.MemorySizeInBy, metadata.AttributeCacheNameQuery)

		r.mb.RecordElasticsearchNodeCacheEvictionsDataPoint(now, info.Indices.FieldDataCache.Evictions, metadata.AttributeCacheNameFielddata)
		r.mb.RecordElasticsearchNodeCacheEvictionsDataPoint(now, info.Indices.QueryCache.Evictions, metadata.AttributeCacheNameQuery)

		r.mb.RecordElasticsearchNodeCacheCountDataPoint(now, info.Indices.QueryCache.HitCount, metadata.AttributeQueryCacheCountTypeHit)
		r.mb.RecordElasticsearchNodeCacheCountDataPoint(now, info.Indices.QueryCache.MissCount, metadata.AttributeQueryCacheCountTypeMiss)

		r.mb.RecordElasticsearchNodeFsDiskAvailableDataPoint(now, info.FS.Total.AvailableBytes)
		r.mb.RecordElasticsearchNodeFsDiskFreeDataPoint(now, info.FS.Total.FreeBytes)
		r.mb.RecordElasticsearchNodeFsDiskTotalDataPoint(now, info.FS.Total.TotalBytes)

		r.mb.RecordElasticsearchNodeDiskIoReadDataPoint(now, info.FS.IOStats.Total.ReadBytes)
		r.mb.RecordElasticsearchNodeDiskIoWriteDataPoint(now, info.FS.IOStats.Total.WriteBytes)

		if r.emitMetricsWithDirectionAttribute {
			r.mb.RecordElasticsearchNodeClusterIoDataPoint(now, info.TransportStats.ReceivedBytes, metadata.AttributeDirectionReceived)
			r.mb.RecordElasticsearchNodeClusterIoDataPoint(now, info.TransportStats.SentBytes, metadata.AttributeDirectionSent)
		}

		if r.emitMetricsWithoutDirectionAttribute {
			r.mb.RecordElasticsearchNodeClusterIoReceivedDataPoint(now, info.TransportStats.ReceivedBytes)
			r.mb.RecordElasticsearchNodeClusterIoSentDataPoint(now, info.TransportStats.SentBytes)
		}

		r.mb.RecordElasticsearchNodeClusterConnectionsDataPoint(now, info.TransportStats.OpenConnections)

		r.mb.RecordElasticsearchNodeHTTPConnectionsDataPoint(now, info.HTTPStats.OpenConnections)

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

		r.mb.EmitForResource(metadata.WithElasticsearchClusterName(nodeStats.ClusterName),
			metadata.WithElasticsearchNodeName(info.Name))
	}
}

func (r *elasticsearchScraper) scrapeClusterMetrics(ctx context.Context, now pcommon.Timestamp, errs *scrapererror.ScrapeErrors) {
	if r.cfg.SkipClusterMetrics {
		return
	}

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

	r.mb.EmitForResource(metadata.WithElasticsearchClusterName(clusterHealth.ClusterName))
}
