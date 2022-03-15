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

import "go.opentelemetry.io/collector/model/pdata"

func (rb *ResourceBuilder) EmitNodeMetrics(metrics pdata.MetricSlice) {
	rb.metricElasticsearchNodeCacheEvictions.emit(metrics)
	rb.metricElasticsearchNodeCacheMemoryUsage.emit(metrics)
	rb.metricElasticsearchNodeClusterConnections.emit(metrics)
	rb.metricElasticsearchNodeClusterIo.emit(metrics)
	rb.metricElasticsearchNodeDocuments.emit(metrics)
	rb.metricElasticsearchNodeFsDiskAvailable.emit(metrics)
	rb.metricElasticsearchNodeHTTPConnections.emit(metrics)
	rb.metricElasticsearchNodeOpenFiles.emit(metrics)
	rb.metricElasticsearchNodeOperationsCompleted.emit(metrics)
	rb.metricElasticsearchNodeOperationsTime.emit(metrics)
	rb.metricElasticsearchNodeShardsSize.emit(metrics)
	rb.metricElasticsearchNodeThreadPoolTasksFinished.emit(metrics)
	rb.metricElasticsearchNodeThreadPoolTasksQueued.emit(metrics)
	rb.metricElasticsearchNodeThreadPoolThreads.emit(metrics)
	rb.metricJvmClassesLoaded.emit(metrics)
	rb.metricJvmGcCollectionsCount.emit(metrics)
	rb.metricJvmGcCollectionsElapsed.emit(metrics)
	rb.metricJvmMemoryHeapCommitted.emit(metrics)
	rb.metricJvmMemoryHeapMax.emit(metrics)
	rb.metricJvmMemoryHeapUsed.emit(metrics)
	rb.metricJvmMemoryNonheapCommitted.emit(metrics)
	rb.metricJvmMemoryNonheapUsed.emit(metrics)
	rb.metricJvmMemoryPoolMax.emit(metrics)
	rb.metricJvmMemoryPoolUsed.emit(metrics)
	rb.metricJvmThreadsCount.emit(metrics)
}

func (rb *ResourceBuilder) EmitClusterMetrics(metrics pdata.MetricSlice) {
	rb.metricElasticsearchClusterDataNodes.emit(metrics)
	rb.metricElasticsearchClusterNodes.emit(metrics)
	rb.metricElasticsearchClusterShards.emit(metrics)
	rb.metricElasticsearchClusterHealth.emit(metrics)
}
