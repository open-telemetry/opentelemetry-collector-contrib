package elasticsearchreceiver

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/elasticsearchreceiver/internal/metadata"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/model/pdata"
	"go.opentelemetry.io/collector/receiver/scrapererror"
	"go.uber.org/multierr"
	"go.uber.org/zap"
)

const instrumentationLibraryName = "otel/elasticsearch"

var errUnknownClusterStatus = errors.New("unknown cluster status")

type elasticsearchScraper struct {
	client         elasticsearchClient
	logger         *zap.Logger
	cfg            *Config
	metricsBuilder *metadata.MetricsBuilder
	now            pdata.Timestamp
}

func newElasticSearchScraper(
	logger *zap.Logger,
	cfg *Config,
) *elasticsearchScraper {
	return &elasticsearchScraper{
		logger:         logger,
		cfg:            cfg,
		now:            pdata.NewTimestampFromTime(time.Now()),
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

	scrapeNodeErr := r.scrapeNodeMetrics(ctx, rms)
	scrapeClusterErr := r.scrapeClusterMetrics(ctx, rms)

	return metrics, multierr.Combine(scrapeClusterErr, scrapeNodeErr)
}

// scrapeNodeMetrics scrapes adds node-level metrics to the given MetricSlice from the NodeStats endpoint
func (r *elasticsearchScraper) scrapeNodeMetrics(ctx context.Context, rms pdata.ResourceMetricsSlice) error {
	if len(r.cfg.Nodes) == 0 {
		return nil
	}

	nodeStats, err := r.client.NodeStats(ctx, r.cfg.Nodes)
	if err != nil {
		return scrapererror.NewPartialScrapeError(err, 26)
	}

	for _, info := range nodeStats.Nodes {
		rm := rms.AppendEmpty()
		resourceAttrs := rm.Resource().Attributes()
		resourceAttrs.InsertString(metadata.A.ElasticsearchClusterName, nodeStats.ClusterName)
		resourceAttrs.InsertString(metadata.A.ElasticsearchNodeName, info.Name)

		ills := rm.InstrumentationLibraryMetrics().AppendEmpty()
		ills.InstrumentationLibrary().SetName(instrumentationLibraryName)

		r.metricsBuilder.RecordElasticsearchNodeCacheMemoryUsageDataPoint(r.now, info.Indices.FieldDataCache.MemorySizeInBy, metadata.AttributeCacheName.Fielddata)
		r.metricsBuilder.RecordElasticsearchNodeCacheMemoryUsageDataPoint(r.now, info.Indices.QueryCache.MemorySizeInBy, metadata.AttributeCacheName.Query)

		r.metricsBuilder.RecordElasticsearchNodeCacheEvictionsDataPoint(r.now, info.Indices.FieldDataCache.Evictions, metadata.AttributeCacheName.Fielddata)
		r.metricsBuilder.RecordElasticsearchNodeCacheEvictionsDataPoint(r.now, info.Indices.QueryCache.Evictions, metadata.AttributeCacheName.Query)

		r.metricsBuilder.RecordElasticsearchNodeFsDiskAvailableDataPoint(r.now, info.FS.Total.AvailableBytes)

		r.metricsBuilder.RecordElasticsearchNodeClusterIoDataPoint(r.now, info.TransportStats.ReceivedBytes, metadata.AttributeDirection.Received)
		r.metricsBuilder.RecordElasticsearchNodeClusterIoDataPoint(r.now, info.TransportStats.SentBytes, metadata.AttributeDirection.Sent)

		r.metricsBuilder.RecordElasticsearchNodeClusterConnectionsDataPoint(r.now, info.TransportStats.OpenConnections)

		r.metricsBuilder.RecordElasticsearchNodeHTTPConnectionsDataPoint(r.now, info.HTTPStats.OpenConnections)

		r.metricsBuilder.RecordElasticsearchNodeOperationsCompletedDataPoint(r.now, info.Indices.IndexingOperations.IndexTotal, metadata.AttributeOperation.Index)
		r.metricsBuilder.RecordElasticsearchNodeOperationsCompletedDataPoint(r.now, info.Indices.IndexingOperations.DeleteTotal, metadata.AttributeOperation.Delete)
		r.metricsBuilder.RecordElasticsearchNodeOperationsCompletedDataPoint(r.now, info.Indices.GetOperation.Total, metadata.AttributeOperation.Get)
		r.metricsBuilder.RecordElasticsearchNodeOperationsCompletedDataPoint(r.now, info.Indices.SearchOperations.QueryTotal, metadata.AttributeOperation.Query)
		r.metricsBuilder.RecordElasticsearchNodeOperationsCompletedDataPoint(r.now, info.Indices.SearchOperations.FetchTotal, metadata.AttributeOperation.Fetch)
		r.metricsBuilder.RecordElasticsearchNodeOperationsCompletedDataPoint(r.now, info.Indices.SearchOperations.ScrollTotal, metadata.AttributeOperation.Scroll)
		r.metricsBuilder.RecordElasticsearchNodeOperationsCompletedDataPoint(r.now, info.Indices.SearchOperations.SuggestTotal, metadata.AttributeOperation.Suggest)
		r.metricsBuilder.RecordElasticsearchNodeOperationsCompletedDataPoint(r.now, info.Indices.MergeOperations.Total, metadata.AttributeOperation.Merge)
		r.metricsBuilder.RecordElasticsearchNodeOperationsCompletedDataPoint(r.now, info.Indices.RefreshOperations.Total, metadata.AttributeOperation.Refresh)
		r.metricsBuilder.RecordElasticsearchNodeOperationsCompletedDataPoint(r.now, info.Indices.FlushOperations.Total, metadata.AttributeOperation.Flush)
		r.metricsBuilder.RecordElasticsearchNodeOperationsCompletedDataPoint(r.now, info.Indices.WarmerOperations.Total, metadata.AttributeOperation.Warmer)

		r.metricsBuilder.RecordElasticsearchNodeOperationsTimeDataPoint(r.now, info.Indices.IndexingOperations.IndexTimeInMs, metadata.AttributeOperation.Index)
		r.metricsBuilder.RecordElasticsearchNodeOperationsTimeDataPoint(r.now, info.Indices.IndexingOperations.DeleteTimeInMs, metadata.AttributeOperation.Delete)
		r.metricsBuilder.RecordElasticsearchNodeOperationsTimeDataPoint(r.now, info.Indices.GetOperation.TotalTimeInMs, metadata.AttributeOperation.Get)
		r.metricsBuilder.RecordElasticsearchNodeOperationsTimeDataPoint(r.now, info.Indices.SearchOperations.QueryTimeInMs, metadata.AttributeOperation.Query)
		r.metricsBuilder.RecordElasticsearchNodeOperationsTimeDataPoint(r.now, info.Indices.SearchOperations.FetchTimeInMs, metadata.AttributeOperation.Fetch)
		r.metricsBuilder.RecordElasticsearchNodeOperationsTimeDataPoint(r.now, info.Indices.SearchOperations.ScrollTimeInMs, metadata.AttributeOperation.Scroll)
		r.metricsBuilder.RecordElasticsearchNodeOperationsTimeDataPoint(r.now, info.Indices.SearchOperations.SuggestTimeInMs, metadata.AttributeOperation.Suggest)
		r.metricsBuilder.RecordElasticsearchNodeOperationsTimeDataPoint(r.now, info.Indices.MergeOperations.TotalTimeInMs, metadata.AttributeOperation.Merge)
		r.metricsBuilder.RecordElasticsearchNodeOperationsTimeDataPoint(r.now, info.Indices.RefreshOperations.TotalTimeInMs, metadata.AttributeOperation.Refresh)
		r.metricsBuilder.RecordElasticsearchNodeOperationsTimeDataPoint(r.now, info.Indices.FlushOperations.TotalTimeInMs, metadata.AttributeOperation.Flush)
		r.metricsBuilder.RecordElasticsearchNodeOperationsTimeDataPoint(r.now, info.Indices.WarmerOperations.TotalTimeInMs, metadata.AttributeOperation.Warmer)

		r.metricsBuilder.RecordElasticsearchNodeShardsSizeDataPoint(r.now, info.Indices.StoreInfo.SizeInBy)

		for tpName, tpInfo := range info.ThreadPoolInfo {
			r.metricsBuilder.RecordElasticsearchNodeThreadPoolThreadsDataPoint(r.now, tpInfo.ActiveThreads, tpName, metadata.AttributeThreadState.Active)
			r.metricsBuilder.RecordElasticsearchNodeThreadPoolThreadsDataPoint(r.now, tpInfo.TotalThreads-tpInfo.ActiveThreads, tpName, metadata.AttributeThreadState.Idle)

			r.metricsBuilder.RecordElasticsearchNodeThreadPoolTasksQueuedDataPoint(r.now, tpInfo.QueuedTasks, tpName)

			r.metricsBuilder.RecordElasticsearchNodeThreadPoolTasksFinishedDataPoint(r.now, tpInfo.CompletedTasks, tpName, metadata.AttributeTaskState.Completed)
			r.metricsBuilder.RecordElasticsearchNodeThreadPoolTasksFinishedDataPoint(r.now, tpInfo.RejectedTasks, tpName, metadata.AttributeTaskState.Rejected)
		}

		r.metricsBuilder.RecordElasticsearchNodeDocumentsDataPoint(r.now, info.Indices.DocumentStats.ActiveCount, metadata.AttributeDocumentState.Active)
		r.metricsBuilder.RecordElasticsearchNodeDocumentsDataPoint(r.now, info.Indices.DocumentStats.DeletedCount, metadata.AttributeDocumentState.Deleted)

		r.metricsBuilder.RecordElasticsearchNodeOpenFilesDataPoint(r.now, info.ProcessStats.OpenFileDescriptorsCount)

		r.metricsBuilder.RecordJvmClassesLoadedDataPoint(r.now, info.JVMInfo.ClassInfo.CurrentLoadedCount)

		r.metricsBuilder.RecordJvmGcCollectionsCountDataPoint(r.now, info.JVMInfo.JVMGCInfo.Collectors.Young.CollectionCount, "young")
		r.metricsBuilder.RecordJvmGcCollectionsCountDataPoint(r.now, info.JVMInfo.JVMGCInfo.Collectors.Old.CollectionCount, "old")

		r.metricsBuilder.RecordJvmGcCollectionsElapsedDataPoint(r.now, info.JVMInfo.JVMGCInfo.Collectors.Young.CollectionTimeInMillis, "young")
		r.metricsBuilder.RecordJvmGcCollectionsElapsedDataPoint(r.now, info.JVMInfo.JVMGCInfo.Collectors.Old.CollectionTimeInMillis, "old")

		r.metricsBuilder.RecordJvmMemoryHeapMaxDataPoint(r.now, info.JVMInfo.JVMMemoryInfo.MaxHeapInBy)
		r.metricsBuilder.RecordJvmMemoryHeapUsedDataPoint(r.now, info.JVMInfo.JVMMemoryInfo.HeapUsedInBy)
		r.metricsBuilder.RecordJvmMemoryHeapCommittedDataPoint(r.now, info.JVMInfo.JVMMemoryInfo.HeapCommittedInBy)

		r.metricsBuilder.RecordJvmMemoryNonheapUsedDataPoint(r.now, info.JVMInfo.JVMMemoryInfo.NonHeapUsedInBy)
		r.metricsBuilder.RecordJvmMemoryNonheapCommittedDataPoint(r.now, info.JVMInfo.JVMMemoryInfo.NonHeapComittedInBy)

		r.metricsBuilder.RecordJvmMemoryPoolUsedDataPoint(r.now, info.JVMInfo.JVMMemoryInfo.MemoryPools.Young.MemUsedBy, "young")
		r.metricsBuilder.RecordJvmMemoryPoolUsedDataPoint(r.now, info.JVMInfo.JVMMemoryInfo.MemoryPools.Survivor.MemUsedBy, "survivor")
		r.metricsBuilder.RecordJvmMemoryPoolUsedDataPoint(r.now, info.JVMInfo.JVMMemoryInfo.MemoryPools.Old.MemUsedBy, "old")

		r.metricsBuilder.RecordJvmMemoryPoolMaxDataPoint(r.now, info.JVMInfo.JVMMemoryInfo.MemoryPools.Young.MemMaxBy, "young")
		r.metricsBuilder.RecordJvmMemoryPoolMaxDataPoint(r.now, info.JVMInfo.JVMMemoryInfo.MemoryPools.Survivor.MemMaxBy, "survivor")
		r.metricsBuilder.RecordJvmMemoryPoolMaxDataPoint(r.now, info.JVMInfo.JVMMemoryInfo.MemoryPools.Old.MemMaxBy, "old")

		r.metricsBuilder.RecordJvmThreadsCountDataPoint(r.now, info.JVMInfo.JVMThreadInfo.Count)

		r.metricsBuilder.EmitNodeMetrics(ills.Metrics())
	}

	return nil
}

func (r *elasticsearchScraper) scrapeClusterMetrics(ctx context.Context, rms pdata.ResourceMetricsSlice) error {
	if r.cfg.SkipClusterMetrics {
		return nil
	}

	clusterHealth, err := r.client.ClusterHealth(ctx)
	if err != nil {
		return scrapererror.NewPartialScrapeError(err, 4)
	}

	rm := rms.AppendEmpty()
	resourceAttrs := rm.Resource().Attributes()
	resourceAttrs.InsertString(metadata.A.ElasticsearchClusterName, clusterHealth.ClusterName)

	ills := rm.InstrumentationLibraryMetrics().AppendEmpty()
	ills.InstrumentationLibrary().SetName(instrumentationLibraryName)

	r.metricsBuilder.RecordElasticsearchClusterNodesDataPoint(r.now, clusterHealth.NodeCount)

	r.metricsBuilder.RecordElasticsearchClusterDataNodesDataPoint(r.now, clusterHealth.DataNodeCount)

	r.metricsBuilder.RecordElasticsearchClusterShardsDataPoint(r.now, clusterHealth.ActiveShards, metadata.AttributeShardState.Active)
	r.metricsBuilder.RecordElasticsearchClusterShardsDataPoint(r.now, clusterHealth.InitializingShards, metadata.AttributeShardState.Initializing)
	r.metricsBuilder.RecordElasticsearchClusterShardsDataPoint(r.now, clusterHealth.RelocatingShards, metadata.AttributeShardState.Relocating)
	r.metricsBuilder.RecordElasticsearchClusterShardsDataPoint(r.now, clusterHealth.UnassignedShards, metadata.AttributeShardState.Unassigned)

	var statusErr error
	switch clusterHealth.Status {
	case "green":
		r.metricsBuilder.RecordElasticsearchClusterHealthDataPoint(r.now, 1, metadata.AttributeHealthStatus.Green)
		r.metricsBuilder.RecordElasticsearchClusterHealthDataPoint(r.now, 0, metadata.AttributeHealthStatus.Yellow)
		r.metricsBuilder.RecordElasticsearchClusterHealthDataPoint(r.now, 0, metadata.AttributeHealthStatus.Red)
	case "yellow":
		r.metricsBuilder.RecordElasticsearchClusterHealthDataPoint(r.now, 0, metadata.AttributeHealthStatus.Green)
		r.metricsBuilder.RecordElasticsearchClusterHealthDataPoint(r.now, 1, metadata.AttributeHealthStatus.Yellow)
		r.metricsBuilder.RecordElasticsearchClusterHealthDataPoint(r.now, 0, metadata.AttributeHealthStatus.Red)
	case "red":
		r.metricsBuilder.RecordElasticsearchClusterHealthDataPoint(r.now, 0, metadata.AttributeHealthStatus.Green)
		r.metricsBuilder.RecordElasticsearchClusterHealthDataPoint(r.now, 0, metadata.AttributeHealthStatus.Yellow)
		r.metricsBuilder.RecordElasticsearchClusterHealthDataPoint(r.now, 1, metadata.AttributeHealthStatus.Red)
	default:
		statusErr = scrapererror.NewPartialScrapeError(fmt.Errorf("health status %s: %w", clusterHealth.Status, errUnknownClusterStatus), 1)
	}

	r.metricsBuilder.EmitClusterMetrics(ills.Metrics())

	return statusErr
}
