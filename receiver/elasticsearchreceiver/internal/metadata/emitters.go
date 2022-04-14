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

package metadata // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/elasticsearchreceiver/internal/metadata"

import "go.opentelemetry.io/collector/pdata/pmetric"

func (mb *MetricsBuilder) EmitNodeMetrics(metrics pmetric.MetricSlice) {
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

func (mb *MetricsBuilder) EmitClusterMetrics(metrics pmetric.MetricSlice) {
	mb.metricElasticsearchClusterDataNodes.emit(metrics)
	mb.metricElasticsearchClusterNodes.emit(metrics)
	mb.metricElasticsearchClusterShards.emit(metrics)
	mb.metricElasticsearchClusterHealth.emit(metrics)
}
