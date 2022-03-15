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
	"go.opentelemetry.io/collector/model/pdata"
	"go.opentelemetry.io/collector/receiver/scrapererror"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/elasticsearchreceiver/internal/metadata"
)

const instrumentationLibraryName = "otelcol/elasticsearch"

var errUnknownClusterStatus = errors.New("unknown cluster status")

type elasticsearchScraper struct {
	client   elasticsearchClient
	settings component.TelemetrySettings
	cfg      *Config
	mb       *metadata.MetricsBuilder
}

func newElasticSearchScraper(
	settings component.TelemetrySettings,
	cfg *Config,
) *elasticsearchScraper {
	return &elasticsearchScraper{
		settings: settings,
		cfg:      cfg,
		mb:       metadata.NewMetricsBuilder(cfg.Metrics),
	}
}

func (r *elasticsearchScraper) start(_ context.Context, host component.Host) (err error) {
	r.client, err = newElasticsearchClient(r.settings, *r.cfg, host)
	return
}

func (r *elasticsearchScraper) scrape(ctx context.Context) (pdata.Metrics, error) {
	errs := &scrapererror.ScrapeErrors{}

	now := pdata.NewTimestampFromTime(time.Now())

	r.scrapeNodeMetrics(ctx, now, errs)
	r.scrapeClusterMetrics(ctx, now, errs)

	return r.mb.Emit(), errs.Combine()
}

// scrapeNodeMetrics scrapes adds node-level metrics to the given MetricSlice from the NodeStats endpoint
func (r *elasticsearchScraper) scrapeNodeMetrics(ctx context.Context, now pdata.Timestamp, errs *scrapererror.ScrapeErrors) {
	if len(r.cfg.Nodes) == 0 {
		return
	}

	nodeStats, err := r.client.NodeStats(ctx, r.cfg.Nodes)
	if err != nil {
		errs.AddPartial(26, err)
		return
	}

	for _, info := range nodeStats.Nodes {
		rb := r.mb.NewResourceBuilder()
		resourceAttrs := rb.Attributes()
		resourceAttrs.InsertString(metadata.A.ElasticsearchClusterName, nodeStats.ClusterName)
		resourceAttrs.InsertString(metadata.A.ElasticsearchNodeName, info.Name)

		rb.RecordElasticsearchNodeCacheMemoryUsageDataPoint(now, info.Indices.FieldDataCache.MemorySizeInBy, metadata.AttributeCacheName.Fielddata)
		rb.RecordElasticsearchNodeCacheMemoryUsageDataPoint(now, info.Indices.QueryCache.MemorySizeInBy, metadata.AttributeCacheName.Query)

		rb.RecordElasticsearchNodeCacheEvictionsDataPoint(now, info.Indices.FieldDataCache.Evictions, metadata.AttributeCacheName.Fielddata)
		rb.RecordElasticsearchNodeCacheEvictionsDataPoint(now, info.Indices.QueryCache.Evictions, metadata.AttributeCacheName.Query)

		rb.RecordElasticsearchNodeFsDiskAvailableDataPoint(now, info.FS.Total.AvailableBytes)

		rb.RecordElasticsearchNodeClusterIoDataPoint(now, info.TransportStats.ReceivedBytes, metadata.AttributeDirection.Received)
		rb.RecordElasticsearchNodeClusterIoDataPoint(now, info.TransportStats.SentBytes, metadata.AttributeDirection.Sent)

		rb.RecordElasticsearchNodeClusterConnectionsDataPoint(now, info.TransportStats.OpenConnections)

		rb.RecordElasticsearchNodeHTTPConnectionsDataPoint(now, info.HTTPStats.OpenConnections)

		rb.RecordElasticsearchNodeOperationsCompletedDataPoint(now, info.Indices.IndexingOperations.IndexTotal, metadata.AttributeOperation.Index)
		rb.RecordElasticsearchNodeOperationsCompletedDataPoint(now, info.Indices.IndexingOperations.DeleteTotal, metadata.AttributeOperation.Delete)
		rb.RecordElasticsearchNodeOperationsCompletedDataPoint(now, info.Indices.GetOperation.Total, metadata.AttributeOperation.Get)
		rb.RecordElasticsearchNodeOperationsCompletedDataPoint(now, info.Indices.SearchOperations.QueryTotal, metadata.AttributeOperation.Query)
		rb.RecordElasticsearchNodeOperationsCompletedDataPoint(now, info.Indices.SearchOperations.FetchTotal, metadata.AttributeOperation.Fetch)
		rb.RecordElasticsearchNodeOperationsCompletedDataPoint(now, info.Indices.SearchOperations.ScrollTotal, metadata.AttributeOperation.Scroll)
		rb.RecordElasticsearchNodeOperationsCompletedDataPoint(now, info.Indices.SearchOperations.SuggestTotal, metadata.AttributeOperation.Suggest)
		rb.RecordElasticsearchNodeOperationsCompletedDataPoint(now, info.Indices.MergeOperations.Total, metadata.AttributeOperation.Merge)
		rb.RecordElasticsearchNodeOperationsCompletedDataPoint(now, info.Indices.RefreshOperations.Total, metadata.AttributeOperation.Refresh)
		rb.RecordElasticsearchNodeOperationsCompletedDataPoint(now, info.Indices.FlushOperations.Total, metadata.AttributeOperation.Flush)
		rb.RecordElasticsearchNodeOperationsCompletedDataPoint(now, info.Indices.WarmerOperations.Total, metadata.AttributeOperation.Warmer)

		rb.RecordElasticsearchNodeOperationsTimeDataPoint(now, info.Indices.IndexingOperations.IndexTimeInMs, metadata.AttributeOperation.Index)
		rb.RecordElasticsearchNodeOperationsTimeDataPoint(now, info.Indices.IndexingOperations.DeleteTimeInMs, metadata.AttributeOperation.Delete)
		rb.RecordElasticsearchNodeOperationsTimeDataPoint(now, info.Indices.GetOperation.TotalTimeInMs, metadata.AttributeOperation.Get)
		rb.RecordElasticsearchNodeOperationsTimeDataPoint(now, info.Indices.SearchOperations.QueryTimeInMs, metadata.AttributeOperation.Query)
		rb.RecordElasticsearchNodeOperationsTimeDataPoint(now, info.Indices.SearchOperations.FetchTimeInMs, metadata.AttributeOperation.Fetch)
		rb.RecordElasticsearchNodeOperationsTimeDataPoint(now, info.Indices.SearchOperations.ScrollTimeInMs, metadata.AttributeOperation.Scroll)
		rb.RecordElasticsearchNodeOperationsTimeDataPoint(now, info.Indices.SearchOperations.SuggestTimeInMs, metadata.AttributeOperation.Suggest)
		rb.RecordElasticsearchNodeOperationsTimeDataPoint(now, info.Indices.MergeOperations.TotalTimeInMs, metadata.AttributeOperation.Merge)
		rb.RecordElasticsearchNodeOperationsTimeDataPoint(now, info.Indices.RefreshOperations.TotalTimeInMs, metadata.AttributeOperation.Refresh)
		rb.RecordElasticsearchNodeOperationsTimeDataPoint(now, info.Indices.FlushOperations.TotalTimeInMs, metadata.AttributeOperation.Flush)
		rb.RecordElasticsearchNodeOperationsTimeDataPoint(now, info.Indices.WarmerOperations.TotalTimeInMs, metadata.AttributeOperation.Warmer)

		rb.RecordElasticsearchNodeShardsSizeDataPoint(now, info.Indices.StoreInfo.SizeInBy)

		for tpName, tpInfo := range info.ThreadPoolInfo {
			rb.RecordElasticsearchNodeThreadPoolThreadsDataPoint(now, tpInfo.ActiveThreads, tpName, metadata.AttributeThreadState.Active)
			rb.RecordElasticsearchNodeThreadPoolThreadsDataPoint(now, tpInfo.TotalThreads-tpInfo.ActiveThreads, tpName, metadata.AttributeThreadState.Idle)

			rb.RecordElasticsearchNodeThreadPoolTasksQueuedDataPoint(now, tpInfo.QueuedTasks, tpName)

			rb.RecordElasticsearchNodeThreadPoolTasksFinishedDataPoint(now, tpInfo.CompletedTasks, tpName, metadata.AttributeTaskState.Completed)
			rb.RecordElasticsearchNodeThreadPoolTasksFinishedDataPoint(now, tpInfo.RejectedTasks, tpName, metadata.AttributeTaskState.Rejected)
		}

		rb.RecordElasticsearchNodeDocumentsDataPoint(now, info.Indices.DocumentStats.ActiveCount, metadata.AttributeDocumentState.Active)
		rb.RecordElasticsearchNodeDocumentsDataPoint(now, info.Indices.DocumentStats.DeletedCount, metadata.AttributeDocumentState.Deleted)

		rb.RecordElasticsearchNodeOpenFilesDataPoint(now, info.ProcessStats.OpenFileDescriptorsCount)

		rb.RecordJvmClassesLoadedDataPoint(now, info.JVMInfo.ClassInfo.CurrentLoadedCount)

		rb.RecordJvmGcCollectionsCountDataPoint(now, info.JVMInfo.JVMGCInfo.Collectors.Young.CollectionCount, "young")
		rb.RecordJvmGcCollectionsCountDataPoint(now, info.JVMInfo.JVMGCInfo.Collectors.Old.CollectionCount, "old")

		rb.RecordJvmGcCollectionsElapsedDataPoint(now, info.JVMInfo.JVMGCInfo.Collectors.Young.CollectionTimeInMillis, "young")
		rb.RecordJvmGcCollectionsElapsedDataPoint(now, info.JVMInfo.JVMGCInfo.Collectors.Old.CollectionTimeInMillis, "old")

		rb.RecordJvmMemoryHeapMaxDataPoint(now, info.JVMInfo.JVMMemoryInfo.MaxHeapInBy)
		rb.RecordJvmMemoryHeapUsedDataPoint(now, info.JVMInfo.JVMMemoryInfo.HeapUsedInBy)
		rb.RecordJvmMemoryHeapCommittedDataPoint(now, info.JVMInfo.JVMMemoryInfo.HeapCommittedInBy)

		rb.RecordJvmMemoryNonheapUsedDataPoint(now, info.JVMInfo.JVMMemoryInfo.NonHeapUsedInBy)
		rb.RecordJvmMemoryNonheapCommittedDataPoint(now, info.JVMInfo.JVMMemoryInfo.NonHeapComittedInBy)

		rb.RecordJvmMemoryPoolUsedDataPoint(now, info.JVMInfo.JVMMemoryInfo.MemoryPools.Young.MemUsedBy, "young")
		rb.RecordJvmMemoryPoolUsedDataPoint(now, info.JVMInfo.JVMMemoryInfo.MemoryPools.Survivor.MemUsedBy, "survivor")
		rb.RecordJvmMemoryPoolUsedDataPoint(now, info.JVMInfo.JVMMemoryInfo.MemoryPools.Old.MemUsedBy, "old")

		rb.RecordJvmMemoryPoolMaxDataPoint(now, info.JVMInfo.JVMMemoryInfo.MemoryPools.Young.MemMaxBy, "young")
		rb.RecordJvmMemoryPoolMaxDataPoint(now, info.JVMInfo.JVMMemoryInfo.MemoryPools.Survivor.MemMaxBy, "survivor")
		rb.RecordJvmMemoryPoolMaxDataPoint(now, info.JVMInfo.JVMMemoryInfo.MemoryPools.Old.MemMaxBy, "old")

		rb.RecordJvmThreadsCountDataPoint(now, info.JVMInfo.JVMThreadInfo.Count)
	}
}

func (r *elasticsearchScraper) scrapeClusterMetrics(ctx context.Context, now pdata.Timestamp, errs *scrapererror.ScrapeErrors) {
	if r.cfg.SkipClusterMetrics {
		return
	}

	clusterHealth, err := r.client.ClusterHealth(ctx)
	if err != nil {
		errs.AddPartial(4, err)
		return
	}

	rb := r.mb.NewResourceBuilder()
	resourceAttrs := rb.Attributes()
	resourceAttrs.InsertString(metadata.A.ElasticsearchClusterName, clusterHealth.ClusterName)

	rb.RecordElasticsearchClusterNodesDataPoint(now, clusterHealth.NodeCount)

	rb.RecordElasticsearchClusterDataNodesDataPoint(now, clusterHealth.DataNodeCount)

	rb.RecordElasticsearchClusterShardsDataPoint(now, clusterHealth.ActiveShards, metadata.AttributeShardState.Active)
	rb.RecordElasticsearchClusterShardsDataPoint(now, clusterHealth.InitializingShards, metadata.AttributeShardState.Initializing)
	rb.RecordElasticsearchClusterShardsDataPoint(now, clusterHealth.RelocatingShards, metadata.AttributeShardState.Relocating)
	rb.RecordElasticsearchClusterShardsDataPoint(now, clusterHealth.UnassignedShards, metadata.AttributeShardState.Unassigned)

	switch clusterHealth.Status {
	case "green":
		rb.RecordElasticsearchClusterHealthDataPoint(now, 1, metadata.AttributeHealthStatus.Green)
		rb.RecordElasticsearchClusterHealthDataPoint(now, 0, metadata.AttributeHealthStatus.Yellow)
		rb.RecordElasticsearchClusterHealthDataPoint(now, 0, metadata.AttributeHealthStatus.Red)
	case "yellow":
		rb.RecordElasticsearchClusterHealthDataPoint(now, 0, metadata.AttributeHealthStatus.Green)
		rb.RecordElasticsearchClusterHealthDataPoint(now, 1, metadata.AttributeHealthStatus.Yellow)
		rb.RecordElasticsearchClusterHealthDataPoint(now, 0, metadata.AttributeHealthStatus.Red)
	case "red":
		rb.RecordElasticsearchClusterHealthDataPoint(now, 0, metadata.AttributeHealthStatus.Green)
		rb.RecordElasticsearchClusterHealthDataPoint(now, 0, metadata.AttributeHealthStatus.Yellow)
		rb.RecordElasticsearchClusterHealthDataPoint(now, 1, metadata.AttributeHealthStatus.Red)
	default:
		errs.AddPartial(1, fmt.Errorf("health status %s: %w", clusterHealth.Status, errUnknownClusterStatus))
	}
}
