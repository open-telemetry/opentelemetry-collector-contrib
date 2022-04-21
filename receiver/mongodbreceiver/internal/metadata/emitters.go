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

package metadata // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbreceiver/internal/metadata"

import (
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func (mb *MetricsBuilder) EmitDatabase(metrics pmetric.MetricSlice) {
	// mb.metricMongodbCacheOperations.emit(metrics)
	mb.metricMongodbCollectionCount.emit(metrics)
	mb.metricMongodbConnectionCount.emit(metrics)
	mb.metricMongodbDataSize.emit(metrics)
	mb.metricMongodbExtentCount.emit(metrics)
	mb.metricMongodbIndexCount.emit(metrics)
	mb.metricMongodbIndexSize.emit(metrics)
	mb.metricMongodbMemoryUsage.emit(metrics)
	mb.metricMongodbObjectCount.emit(metrics)
	// mb.metricMongodbOperationCount.emit(metrics)
	mb.metricMongodbStorageSize.emit(metrics)
}

func (mb *MetricsBuilder) EmitAdmin(metrics pmetric.MetricSlice) {
	mb.metricMongodbGlobalLockTime.emit(metrics)
	mb.metricMongodbOperationCount.emit(metrics)
	mb.metricMongodbCacheOperations.emit(metrics)
}
