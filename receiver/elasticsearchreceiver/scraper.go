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
	metrics := pdata.NewMetrics()
	rms := metrics.ResourceMetrics()

	errs := &scrapererror.ScrapeErrors{}

	now := pdata.NewTimestampFromTime(time.Now())

	r.scrapeNodeMetrics(ctx, now, rms, errs)
	r.scrapeClusterMetrics(ctx, now, rms, errs)

	return metrics, errs.Combine()
}

// scrapeNodeMetrics scrapes adds node-level metrics to the given MetricSlice from the NodeStats endpoint
func (r *elasticsearchScraper) scrapeNodeMetrics(ctx context.Context, now pdata.Timestamp, rms pdata.ResourceMetricsSlice, errs *scrapererror.ScrapeErrors) {
	if len(r.cfg.Nodes) == 0 {
		return
	}

	nodeStats, err := r.client.NodeStats(ctx, r.cfg.Nodes)
	if err != nil {
		errs.AddPartial(26, err)
		return
	}

	for _, info := range nodeStats.Nodes {
		rm := rms.AppendEmpty()
		resourceAttrs := rm.Resource().Attributes()
		resourceAttrs.InsertString(metadata.A.ElasticsearchClusterName, nodeStats.ClusterName)
		resourceAttrs.InsertString(metadata.A.ElasticsearchNodeName, info.Name)

		ilms := rm.ScopeMetrics().AppendEmpty()
		ilms.Scope().SetName(instrumentationLibraryName)

		r.mb.RecordElasticsearchNodeCacheMemoryUsageDataPoint(now, info.Indices.FieldDataCache.MemorySizeInBy, metadata.AttributeCacheName.Fielddata)
		r.mb.RecordElasticsearchNodeCacheMemoryUsageDataPoint(now, info.Indices.QueryCache.MemorySizeInBy, metadata.AttributeCacheName.Query)

		r.mb.RecordElasticsearchNodeCacheEvictionsDataPoint(now, info.Indices.FieldDataCache.Evictions, metadata.AttributeCacheName.Fielddata)
		r.mb.RecordElasticsearchNodeCacheEvictionsDataPoint(now, info.Indices.QueryCache.Evictions, metadata.AttributeCacheName.Query)

		r.mb.RecordElasticsearchNodeFsDiskAvailableDataPoint(now, info.FS.Total.AvailableBytes)

		r.mb.RecordElasticsearchNodeClusterIoDataPoint(now, info.TransportStats.ReceivedBytes, metadata.AttributeDirection.Received)
		r.mb.RecordElasticsearchNodeClusterIoDataPoint(now, info.TransportStats.SentBytes, metadata.AttributeDirection.Sent)

		r.mb.RecordElasticsearchNodeClusterConnectionsDataPoint(now, info.TransportStats.OpenConnections)

		r.mb.RecordElasticsearchNodeHTTPConnectionsDataPoint(now, info.HTTPStats.OpenConnections)

		r.mb.RecordElasticsearchNodeOperationsCompletedDataPoint(now, info.Indices.IndexingOperations.IndexTotal, metadata.AttributeOperation.Index)
		r.mb.RecordElasticsearchNodeOperationsCompletedDataPoint(now, info.Indices.IndexingOperations.DeleteTotal, metadata.AttributeOperation.Delete)
		r.mb.RecordElasticsearchNodeOperationsCompletedDataPoint(now, info.Indices.GetOperation.Total, metadata.AttributeOperation.Get)
		r.mb.RecordElasticsearchNodeOperationsCompletedDataPoint(now, info.Indices.SearchOperations.QueryTotal, metadata.AttributeOperation.Query)
		r.mb.RecordElasticsearchNodeOperationsCompletedDataPoint(now, info.Indices.SearchOperations.FetchTotal, metadata.AttributeOperation.Fetch)
		r.mb.RecordElasticsearchNodeOperationsCompletedDataPoint(now, info.Indices.SearchOperations.ScrollTotal, metadata.AttributeOperation.Scroll)
		r.mb.RecordElasticsearchNodeOperationsCompletedDataPoint(now, info.Indices.SearchOperations.SuggestTotal, metadata.AttributeOperation.Suggest)
		r.mb.RecordElasticsearchNodeOperationsCompletedDataPoint(now, info.Indices.MergeOperations.Total, metadata.AttributeOperation.Merge)
		r.mb.RecordElasticsearchNodeOperationsCompletedDataPoint(now, info.Indices.RefreshOperations.Total, metadata.AttributeOperation.Refresh)
		r.mb.RecordElasticsearchNodeOperationsCompletedDataPoint(now, info.Indices.FlushOperations.Total, metadata.AttributeOperation.Flush)
		r.mb.RecordElasticsearchNodeOperationsCompletedDataPoint(now, info.Indices.WarmerOperations.Total, metadata.AttributeOperation.Warmer)

		r.mb.RecordElasticsearchNodeOperationsTimeDataPoint(now, info.Indices.IndexingOperations.IndexTimeInMs, metadata.AttributeOperation.Index)
		r.mb.RecordElasticsearchNodeOperationsTimeDataPoint(now, info.Indices.IndexingOperations.DeleteTimeInMs, metadata.AttributeOperation.Delete)
		r.mb.RecordElasticsearchNodeOperationsTimeDataPoint(now, info.Indices.GetOperation.TotalTimeInMs, metadata.AttributeOperation.Get)
		r.mb.RecordElasticsearchNodeOperationsTimeDataPoint(now, info.Indices.SearchOperations.QueryTimeInMs, metadata.AttributeOperation.Query)
		r.mb.RecordElasticsearchNodeOperationsTimeDataPoint(now, info.Indices.SearchOperations.FetchTimeInMs, metadata.AttributeOperation.Fetch)
		r.mb.RecordElasticsearchNodeOperationsTimeDataPoint(now, info.Indices.SearchOperations.ScrollTimeInMs, metadata.AttributeOperation.Scroll)
		r.mb.RecordElasticsearchNodeOperationsTimeDataPoint(now, info.Indices.SearchOperations.SuggestTimeInMs, metadata.AttributeOperation.Suggest)
		r.mb.RecordElasticsearchNodeOperationsTimeDataPoint(now, info.Indices.MergeOperations.TotalTimeInMs, metadata.AttributeOperation.Merge)
		r.mb.RecordElasticsearchNodeOperationsTimeDataPoint(now, info.Indices.RefreshOperations.TotalTimeInMs, metadata.AttributeOperation.Refresh)
		r.mb.RecordElasticsearchNodeOperationsTimeDataPoint(now, info.Indices.FlushOperations.TotalTimeInMs, metadata.AttributeOperation.Flush)
		r.mb.RecordElasticsearchNodeOperationsTimeDataPoint(now, info.Indices.WarmerOperations.TotalTimeInMs, metadata.AttributeOperation.Warmer)

		r.mb.RecordElasticsearchNodeShardsSizeDataPoint(now, info.Indices.StoreInfo.SizeInBy)

		for tpName, tpInfo := range info.ThreadPoolInfo {
			r.mb.RecordElasticsearchNodeThreadPoolThreadsDataPoint(now, tpInfo.ActiveThreads, tpName, metadata.AttributeThreadState.Active)
			r.mb.RecordElasticsearchNodeThreadPoolThreadsDataPoint(now, tpInfo.TotalThreads-tpInfo.ActiveThreads, tpName, metadata.AttributeThreadState.Idle)

			r.mb.RecordElasticsearchNodeThreadPoolTasksQueuedDataPoint(now, tpInfo.QueuedTasks, tpName)

			r.mb.RecordElasticsearchNodeThreadPoolTasksFinishedDataPoint(now, tpInfo.CompletedTasks, tpName, metadata.AttributeTaskState.Completed)
			r.mb.RecordElasticsearchNodeThreadPoolTasksFinishedDataPoint(now, tpInfo.RejectedTasks, tpName, metadata.AttributeTaskState.Rejected)
		}

		r.mb.RecordElasticsearchNodeDocumentsDataPoint(now, info.Indices.DocumentStats.ActiveCount, metadata.AttributeDocumentState.Active)
		r.mb.RecordElasticsearchNodeDocumentsDataPoint(now, info.Indices.DocumentStats.DeletedCount, metadata.AttributeDocumentState.Deleted)

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

		r.mb.EmitNodeMetrics(ilms.Metrics())
	}
}

func (r *elasticsearchScraper) scrapeClusterMetrics(ctx context.Context, now pdata.Timestamp, rms pdata.ResourceMetricsSlice, errs *scrapererror.ScrapeErrors) {
	if r.cfg.SkipClusterMetrics {
		return
	}

	clusterHealth, err := r.client.ClusterHealth(ctx)
	if err != nil {
		errs.AddPartial(4, err)
		return
	}

	rm := rms.AppendEmpty()
	resourceAttrs := rm.Resource().Attributes()
	resourceAttrs.InsertString(metadata.A.ElasticsearchClusterName, clusterHealth.ClusterName)

	ilms := rm.ScopeMetrics().AppendEmpty()
	ilms.Scope().SetName(instrumentationLibraryName)

	r.mb.RecordElasticsearchClusterNodesDataPoint(now, clusterHealth.NodeCount)

	r.mb.RecordElasticsearchClusterDataNodesDataPoint(now, clusterHealth.DataNodeCount)

	r.mb.RecordElasticsearchClusterShardsDataPoint(now, clusterHealth.ActiveShards, metadata.AttributeShardState.Active)
	r.mb.RecordElasticsearchClusterShardsDataPoint(now, clusterHealth.InitializingShards, metadata.AttributeShardState.Initializing)
	r.mb.RecordElasticsearchClusterShardsDataPoint(now, clusterHealth.RelocatingShards, metadata.AttributeShardState.Relocating)
	r.mb.RecordElasticsearchClusterShardsDataPoint(now, clusterHealth.UnassignedShards, metadata.AttributeShardState.Unassigned)

	switch clusterHealth.Status {
	case "green":
		r.mb.RecordElasticsearchClusterHealthDataPoint(now, 1, metadata.AttributeHealthStatus.Green)
		r.mb.RecordElasticsearchClusterHealthDataPoint(now, 0, metadata.AttributeHealthStatus.Yellow)
		r.mb.RecordElasticsearchClusterHealthDataPoint(now, 0, metadata.AttributeHealthStatus.Red)
	case "yellow":
		r.mb.RecordElasticsearchClusterHealthDataPoint(now, 0, metadata.AttributeHealthStatus.Green)
		r.mb.RecordElasticsearchClusterHealthDataPoint(now, 1, metadata.AttributeHealthStatus.Yellow)
		r.mb.RecordElasticsearchClusterHealthDataPoint(now, 0, metadata.AttributeHealthStatus.Red)
	case "red":
		r.mb.RecordElasticsearchClusterHealthDataPoint(now, 0, metadata.AttributeHealthStatus.Green)
		r.mb.RecordElasticsearchClusterHealthDataPoint(now, 0, metadata.AttributeHealthStatus.Yellow)
		r.mb.RecordElasticsearchClusterHealthDataPoint(now, 1, metadata.AttributeHealthStatus.Red)
	default:
		errs.AddPartial(1, fmt.Errorf("health status %s: %w", clusterHealth.Status, errUnknownClusterStatus))
	}

	r.mb.EmitClusterMetrics(ilms.Metrics())
}
