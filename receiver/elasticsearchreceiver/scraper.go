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
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/elasticsearchreceiver/internal/metadata"
)

const instrumentationLibraryName = "otelcol/elasticsearch"

var errUnknownClusterStatus = errors.New("unknown cluster status")

type elasticsearchScraper struct {
	client         elasticsearchClient
	logger         *zap.Logger
	cfg            *Config
	metricsBuilder *metadata.MetricsBuilder
}

func newElasticSearchScraper(
	logger *zap.Logger,
	cfg *Config,
) *elasticsearchScraper {
	return &elasticsearchScraper{
		logger:         logger,
		cfg:            cfg,
		metricsBuilder: metadata.NewMetricsBuilder(cfg.Metrics),
	}
}

func (r *elasticsearchScraper) start(_ context.Context, host component.Host) (err error) {
	r.client, err = newElasticsearchClient(r.logger, *r.cfg, host)
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

		ilms := rm.InstrumentationLibraryMetrics().AppendEmpty()
		ilms.InstrumentationLibrary().SetName(instrumentationLibraryName)

		r.metricsBuilder.RecordElasticsearchNodeCacheMemoryUsageDataPoint(now, info.Indices.FieldDataCache.MemorySizeInBy, metadata.AttributeCacheName.Fielddata)
		r.metricsBuilder.RecordElasticsearchNodeCacheMemoryUsageDataPoint(now, info.Indices.QueryCache.MemorySizeInBy, metadata.AttributeCacheName.Query)

		r.metricsBuilder.RecordElasticsearchNodeCacheEvictionsDataPoint(now, info.Indices.FieldDataCache.Evictions, metadata.AttributeCacheName.Fielddata)
		r.metricsBuilder.RecordElasticsearchNodeCacheEvictionsDataPoint(now, info.Indices.QueryCache.Evictions, metadata.AttributeCacheName.Query)

		r.metricsBuilder.RecordElasticsearchNodeFsDiskAvailableDataPoint(now, info.FS.Total.AvailableBytes)

		r.metricsBuilder.RecordElasticsearchNodeClusterIoDataPoint(now, info.TransportStats.ReceivedBytes, metadata.AttributeDirection.Received)
		r.metricsBuilder.RecordElasticsearchNodeClusterIoDataPoint(now, info.TransportStats.SentBytes, metadata.AttributeDirection.Sent)

		r.metricsBuilder.RecordElasticsearchNodeClusterConnectionsDataPoint(now, info.TransportStats.OpenConnections)

		r.metricsBuilder.RecordElasticsearchNodeHTTPConnectionsDataPoint(now, info.HTTPStats.OpenConnections)

		r.metricsBuilder.RecordElasticsearchNodeOperationsCompletedDataPoint(now, info.Indices.IndexingOperations.IndexTotal, metadata.AttributeOperation.Index)
		r.metricsBuilder.RecordElasticsearchNodeOperationsCompletedDataPoint(now, info.Indices.IndexingOperations.DeleteTotal, metadata.AttributeOperation.Delete)
		r.metricsBuilder.RecordElasticsearchNodeOperationsCompletedDataPoint(now, info.Indices.GetOperation.Total, metadata.AttributeOperation.Get)
		r.metricsBuilder.RecordElasticsearchNodeOperationsCompletedDataPoint(now, info.Indices.SearchOperations.QueryTotal, metadata.AttributeOperation.Query)
		r.metricsBuilder.RecordElasticsearchNodeOperationsCompletedDataPoint(now, info.Indices.SearchOperations.FetchTotal, metadata.AttributeOperation.Fetch)
		r.metricsBuilder.RecordElasticsearchNodeOperationsCompletedDataPoint(now, info.Indices.SearchOperations.ScrollTotal, metadata.AttributeOperation.Scroll)
		r.metricsBuilder.RecordElasticsearchNodeOperationsCompletedDataPoint(now, info.Indices.SearchOperations.SuggestTotal, metadata.AttributeOperation.Suggest)
		r.metricsBuilder.RecordElasticsearchNodeOperationsCompletedDataPoint(now, info.Indices.MergeOperations.Total, metadata.AttributeOperation.Merge)
		r.metricsBuilder.RecordElasticsearchNodeOperationsCompletedDataPoint(now, info.Indices.RefreshOperations.Total, metadata.AttributeOperation.Refresh)
		r.metricsBuilder.RecordElasticsearchNodeOperationsCompletedDataPoint(now, info.Indices.FlushOperations.Total, metadata.AttributeOperation.Flush)
		r.metricsBuilder.RecordElasticsearchNodeOperationsCompletedDataPoint(now, info.Indices.WarmerOperations.Total, metadata.AttributeOperation.Warmer)

		r.metricsBuilder.RecordElasticsearchNodeOperationsTimeDataPoint(now, info.Indices.IndexingOperations.IndexTimeInMs, metadata.AttributeOperation.Index)
		r.metricsBuilder.RecordElasticsearchNodeOperationsTimeDataPoint(now, info.Indices.IndexingOperations.DeleteTimeInMs, metadata.AttributeOperation.Delete)
		r.metricsBuilder.RecordElasticsearchNodeOperationsTimeDataPoint(now, info.Indices.GetOperation.TotalTimeInMs, metadata.AttributeOperation.Get)
		r.metricsBuilder.RecordElasticsearchNodeOperationsTimeDataPoint(now, info.Indices.SearchOperations.QueryTimeInMs, metadata.AttributeOperation.Query)
		r.metricsBuilder.RecordElasticsearchNodeOperationsTimeDataPoint(now, info.Indices.SearchOperations.FetchTimeInMs, metadata.AttributeOperation.Fetch)
		r.metricsBuilder.RecordElasticsearchNodeOperationsTimeDataPoint(now, info.Indices.SearchOperations.ScrollTimeInMs, metadata.AttributeOperation.Scroll)
		r.metricsBuilder.RecordElasticsearchNodeOperationsTimeDataPoint(now, info.Indices.SearchOperations.SuggestTimeInMs, metadata.AttributeOperation.Suggest)
		r.metricsBuilder.RecordElasticsearchNodeOperationsTimeDataPoint(now, info.Indices.MergeOperations.TotalTimeInMs, metadata.AttributeOperation.Merge)
		r.metricsBuilder.RecordElasticsearchNodeOperationsTimeDataPoint(now, info.Indices.RefreshOperations.TotalTimeInMs, metadata.AttributeOperation.Refresh)
		r.metricsBuilder.RecordElasticsearchNodeOperationsTimeDataPoint(now, info.Indices.FlushOperations.TotalTimeInMs, metadata.AttributeOperation.Flush)
		r.metricsBuilder.RecordElasticsearchNodeOperationsTimeDataPoint(now, info.Indices.WarmerOperations.TotalTimeInMs, metadata.AttributeOperation.Warmer)

		r.metricsBuilder.RecordElasticsearchNodeShardsSizeDataPoint(now, info.Indices.StoreInfo.SizeInBy)

		for tpName, tpInfo := range info.ThreadPoolInfo {
			r.metricsBuilder.RecordElasticsearchNodeThreadPoolThreadsDataPoint(now, tpInfo.ActiveThreads, tpName, metadata.AttributeThreadState.Active)
			r.metricsBuilder.RecordElasticsearchNodeThreadPoolThreadsDataPoint(now, tpInfo.TotalThreads-tpInfo.ActiveThreads, tpName, metadata.AttributeThreadState.Idle)

			r.metricsBuilder.RecordElasticsearchNodeThreadPoolTasksQueuedDataPoint(now, tpInfo.QueuedTasks, tpName)

			r.metricsBuilder.RecordElasticsearchNodeThreadPoolTasksFinishedDataPoint(now, tpInfo.CompletedTasks, tpName, metadata.AttributeTaskState.Completed)
			r.metricsBuilder.RecordElasticsearchNodeThreadPoolTasksFinishedDataPoint(now, tpInfo.RejectedTasks, tpName, metadata.AttributeTaskState.Rejected)
		}

		r.metricsBuilder.RecordElasticsearchNodeDocumentsDataPoint(now, info.Indices.DocumentStats.ActiveCount, metadata.AttributeDocumentState.Active)
		r.metricsBuilder.RecordElasticsearchNodeDocumentsDataPoint(now, info.Indices.DocumentStats.DeletedCount, metadata.AttributeDocumentState.Deleted)

		r.metricsBuilder.RecordElasticsearchNodeOpenFilesDataPoint(now, info.ProcessStats.OpenFileDescriptorsCount)

		r.metricsBuilder.RecordJvmClassesLoadedDataPoint(now, info.JVMInfo.ClassInfo.CurrentLoadedCount)

		r.metricsBuilder.RecordJvmGcCollectionsCountDataPoint(now, info.JVMInfo.JVMGCInfo.Collectors.Young.CollectionCount, "young")
		r.metricsBuilder.RecordJvmGcCollectionsCountDataPoint(now, info.JVMInfo.JVMGCInfo.Collectors.Old.CollectionCount, "old")

		r.metricsBuilder.RecordJvmGcCollectionsElapsedDataPoint(now, info.JVMInfo.JVMGCInfo.Collectors.Young.CollectionTimeInMillis, "young")
		r.metricsBuilder.RecordJvmGcCollectionsElapsedDataPoint(now, info.JVMInfo.JVMGCInfo.Collectors.Old.CollectionTimeInMillis, "old")

		r.metricsBuilder.RecordJvmMemoryHeapMaxDataPoint(now, info.JVMInfo.JVMMemoryInfo.MaxHeapInBy)
		r.metricsBuilder.RecordJvmMemoryHeapUsedDataPoint(now, info.JVMInfo.JVMMemoryInfo.HeapUsedInBy)
		r.metricsBuilder.RecordJvmMemoryHeapCommittedDataPoint(now, info.JVMInfo.JVMMemoryInfo.HeapCommittedInBy)

		r.metricsBuilder.RecordJvmMemoryNonheapUsedDataPoint(now, info.JVMInfo.JVMMemoryInfo.NonHeapUsedInBy)
		r.metricsBuilder.RecordJvmMemoryNonheapCommittedDataPoint(now, info.JVMInfo.JVMMemoryInfo.NonHeapComittedInBy)

		r.metricsBuilder.RecordJvmMemoryPoolUsedDataPoint(now, info.JVMInfo.JVMMemoryInfo.MemoryPools.Young.MemUsedBy, "young")
		r.metricsBuilder.RecordJvmMemoryPoolUsedDataPoint(now, info.JVMInfo.JVMMemoryInfo.MemoryPools.Survivor.MemUsedBy, "survivor")
		r.metricsBuilder.RecordJvmMemoryPoolUsedDataPoint(now, info.JVMInfo.JVMMemoryInfo.MemoryPools.Old.MemUsedBy, "old")

		r.metricsBuilder.RecordJvmMemoryPoolMaxDataPoint(now, info.JVMInfo.JVMMemoryInfo.MemoryPools.Young.MemMaxBy, "young")
		r.metricsBuilder.RecordJvmMemoryPoolMaxDataPoint(now, info.JVMInfo.JVMMemoryInfo.MemoryPools.Survivor.MemMaxBy, "survivor")
		r.metricsBuilder.RecordJvmMemoryPoolMaxDataPoint(now, info.JVMInfo.JVMMemoryInfo.MemoryPools.Old.MemMaxBy, "old")

		r.metricsBuilder.RecordJvmThreadsCountDataPoint(now, info.JVMInfo.JVMThreadInfo.Count)

		r.metricsBuilder.EmitNodeMetrics(ilms.Metrics())
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

	ilms := rm.InstrumentationLibraryMetrics().AppendEmpty()
	ilms.InstrumentationLibrary().SetName(instrumentationLibraryName)

	r.metricsBuilder.RecordElasticsearchClusterNodesDataPoint(now, clusterHealth.NodeCount)

	r.metricsBuilder.RecordElasticsearchClusterDataNodesDataPoint(now, clusterHealth.DataNodeCount)

	r.metricsBuilder.RecordElasticsearchClusterShardsDataPoint(now, clusterHealth.ActiveShards, metadata.AttributeShardState.Active)
	r.metricsBuilder.RecordElasticsearchClusterShardsDataPoint(now, clusterHealth.InitializingShards, metadata.AttributeShardState.Initializing)
	r.metricsBuilder.RecordElasticsearchClusterShardsDataPoint(now, clusterHealth.RelocatingShards, metadata.AttributeShardState.Relocating)
	r.metricsBuilder.RecordElasticsearchClusterShardsDataPoint(now, clusterHealth.UnassignedShards, metadata.AttributeShardState.Unassigned)

	switch clusterHealth.Status {
	case "green":
		r.metricsBuilder.RecordElasticsearchClusterHealthDataPoint(now, 1, metadata.AttributeHealthStatus.Green)
		r.metricsBuilder.RecordElasticsearchClusterHealthDataPoint(now, 0, metadata.AttributeHealthStatus.Yellow)
		r.metricsBuilder.RecordElasticsearchClusterHealthDataPoint(now, 0, metadata.AttributeHealthStatus.Red)
	case "yellow":
		r.metricsBuilder.RecordElasticsearchClusterHealthDataPoint(now, 0, metadata.AttributeHealthStatus.Green)
		r.metricsBuilder.RecordElasticsearchClusterHealthDataPoint(now, 1, metadata.AttributeHealthStatus.Yellow)
		r.metricsBuilder.RecordElasticsearchClusterHealthDataPoint(now, 0, metadata.AttributeHealthStatus.Red)
	case "red":
		r.metricsBuilder.RecordElasticsearchClusterHealthDataPoint(now, 0, metadata.AttributeHealthStatus.Green)
		r.metricsBuilder.RecordElasticsearchClusterHealthDataPoint(now, 0, metadata.AttributeHealthStatus.Yellow)
		r.metricsBuilder.RecordElasticsearchClusterHealthDataPoint(now, 1, metadata.AttributeHealthStatus.Red)
	default:
		errs.AddPartial(1, fmt.Errorf("health status %s: %w", clusterHealth.Status, errUnknownClusterStatus))
	}

	r.metricsBuilder.EmitClusterMetrics(ilms.Metrics())
}
