package metadata

import "go.opentelemetry.io/collector/model/pdata"

func (mb *MetricsBuilder) EmitNodeMetrics(metrics pdata.MetricSlice) {
	mb.metricElasticsearchNodeCacheEvictions.emit(metrics)
	mb.metricElasticsearchNodeCacheMemoryUsage.emit(metrics)
	mb.metricElasticsearchNodeClusterConnections.emit(metrics)
	mb.metricElasticsearchNodeClusterIo.emit(metrics)
	mb.metricElasticsearchNodeDocuments.emit(metrics)
	mb.metricElasticsearchNodeFsDiskAvailable.emit(metrics)
	mb.metricElasticsearchNodeHTTPConnections.emit(metrics)
	mb.metricElasticsearchNodeOpenFiles.emit(metrics)
	mb.metricElasticsearchNodeOperationsCompleted.emit(metrics)
	mb.metricElasticsearchNodeOperationsTime.emit(metrics)
	mb.metricElasticsearchNodeShardsSize.emit(metrics)
	mb.metricElasticsearchNodeThreadPoolTasksFinished.emit(metrics)
	mb.metricElasticsearchNodeThreadPoolTasksQueued.emit(metrics)
	mb.metricElasticsearchNodeThreadPoolThreads.emit(metrics)
	mb.metricJvmClassesLoaded.emit(metrics)
	mb.metricJvmGcCollectionsCount.emit(metrics)
	mb.metricJvmGcCollectionsElapsed.emit(metrics)
	mb.metricJvmMemoryHeapCommitted.emit(metrics)
	mb.metricJvmMemoryHeapMax.emit(metrics)
	mb.metricJvmMemoryHeapUsed.emit(metrics)
	mb.metricJvmMemoryNonheapCommitted.emit(metrics)
	mb.metricJvmMemoryNonheapUsed.emit(metrics)
	mb.metricJvmMemoryPoolMax.emit(metrics)
	mb.metricJvmMemoryPoolUsed.emit(metrics)
	mb.metricJvmThreadsCount.emit(metrics)
}

func (mb *MetricsBuilder) EmitClusterMetrics(metrics pdata.MetricSlice) {
	mb.metricElasticsearchClusterDataNodes.emit(metrics)
	mb.metricElasticsearchClusterNodes.emit(metrics)
	mb.metricElasticsearchClusterShards.emit(metrics)
	mb.metricElasticsearchClusterHealth.emit(metrics)
}
