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

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/scrapererror"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/elasticsearchreceiver/internal/metadata"
)

var errUnknownClusterStatus = errors.New("unknown cluster status")

type elasticsearchScraper struct {
	client   elasticsearchClient
	settings component.TelemetrySettings
	cfg      *Config
	mb       *metadata.MetricsBuilder
}

func newElasticSearchScraper(
	settings component.ReceiverCreateSettings,
	cfg *Config,
) *elasticsearchScraper {
	return &elasticsearchScraper{
		settings: settings.TelemetrySettings,
		cfg:      cfg,
		mb:       metadata.NewMetricsBuilder(cfg.Metrics, settings.BuildInfo),
	}
}

func (r *elasticsearchScraper) start(_ context.Context, host component.Host) (err error) {
	r.client, err = newElasticsearchClient(r.settings, *r.cfg, host)
	return
}

func (r *elasticsearchScraper) scrape(ctx context.Context) (pmetric.Metrics, error) {
	errs := &scrapererror.ScrapeErrors{}

	now := pcommon.NewTimestampFromTime(time.Now())

	r.scrapeNodeMetrics(ctx, now, errs)
	r.scrapeClusterMetrics(ctx, now, errs)

	return r.mb.Emit(), errs.Combine()
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

		r.mb.RecordElasticsearchNodeFsDiskAvailableDataPoint(now, info.FS.Total.AvailableBytes)

		r.mb.RecordElasticsearchNodeClusterIoDataPoint(now, info.TransportStats.ReceivedBytes, metadata.AttributeDirectionReceived)
		r.mb.RecordElasticsearchNodeClusterIoDataPoint(now, info.TransportStats.SentBytes, metadata.AttributeDirectionSent)

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

		for tpName, tpInfo := range info.ThreadPoolInfo {
			r.mb.RecordElasticsearchNodeThreadPoolThreadsDataPoint(now, tpInfo.ActiveThreads, tpName, metadata.AttributeThreadStateActive)
			r.mb.RecordElasticsearchNodeThreadPoolThreadsDataPoint(now, tpInfo.TotalThreads-tpInfo.ActiveThreads, tpName, metadata.AttributeThreadStateIdle)

			r.mb.RecordElasticsearchNodeThreadPoolTasksQueuedDataPoint(now, tpInfo.QueuedTasks, tpName)

			r.mb.RecordElasticsearchNodeThreadPoolTasksFinishedDataPoint(now, tpInfo.CompletedTasks, tpName, metadata.AttributeTaskStateCompleted)
			r.mb.RecordElasticsearchNodeThreadPoolTasksFinishedDataPoint(now, tpInfo.RejectedTasks, tpName, metadata.AttributeTaskStateRejected)
		}

		r.mb.RecordElasticsearchNodeDocumentsDataPoint(now, info.Indices.DocumentStats.ActiveCount, metadata.AttributeDocumentStateActive)
		r.mb.RecordElasticsearchNodeDocumentsDataPoint(now, info.Indices.DocumentStats.DeletedCount, metadata.AttributeDocumentStateDeleted)

		r.mb.RecordElasticsearchNodeOpenFilesDataPoint(now, info.ProcessStats.OpenFileDescriptorsCount)

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
