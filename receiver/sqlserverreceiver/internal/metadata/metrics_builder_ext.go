// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package metadata // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlserverreceiver/internal/metadata"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func (mb *MetricsBuilder) RecordAnyDataPoint(ts pcommon.Timestamp, val float64, name string, attributes map[string]string) {
	switch name {
	case "sqlserver.user.connection.count":
		mb.RecordSqlserverUserConnectionCountDataPoint(ts, int64(val))
	case "sqlserver.batch.request.rate":
		mb.RecordSqlserverBatchRequestRateDataPoint(ts, val)
	case "sqlserver.batch.sql_compilation.rate":
		mb.RecordSqlserverBatchSQLCompilationRateDataPoint(ts, val)
	case "sqlserver.batch.sql_recompilation.rate":
		mb.RecordSqlserverBatchSQLRecompilationRateDataPoint(ts, val)
	case "sqlserver.lock.wait.rate":
		mb.RecordSqlserverLockWaitRateDataPoint(ts, val)
	case "sqlserver.lock.wait_time.avg":
		mb.RecordSqlserverLockWaitTimeAvgDataPoint(ts, val)
	case "sqlserver.page.buffer_cache.hit_ratio":
		mb.RecordSqlserverPageBufferCacheHitRatioDataPoint(ts, val)
	case "sqlserver.page.checkpoint.flush.rate":
		mb.RecordSqlserverPageCheckpointFlushRateDataPoint(ts, val)
	case "sqlserver.page.lazy_write.rate":
		mb.RecordSqlserverPageLazyWriteRateDataPoint(ts, val)
	case "sqlserver.page.life_expectancy":
		mb.RecordSqlserverPageLifeExpectancyDataPoint(ts, int64(val))
	case "sqlserver.page.operation.rate":
		mb.RecordSqlserverPageOperationRateDataPoint(ts, val, MapAttributePageOperations[attributes["type"]])
	case "sqlserver.page.split.rate":
		mb.RecordSqlserverPageSplitRateDataPoint(ts, val)
	case "sqlserver.transaction_log.flush.data.rate":
		mb.RecordSqlserverTransactionLogFlushDataRateDataPoint(ts, val)
	case "sqlserver.transaction_log.flush.rate":
		mb.RecordSqlserverTransactionLogFlushRateDataPoint(ts, val)
	case "sqlserver.transaction_log.flush.wait.rate":
		mb.RecordSqlserverTransactionLogFlushWaitRateDataPoint(ts, val)
	case "sqlserver.transaction_log.growth.count":
		mb.RecordSqlserverTransactionLogGrowthCountDataPoint(ts, int64(val))
	case "sqlserver.transaction_log.shrink.count":
		mb.RecordSqlserverTransactionLogShrinkCountDataPoint(ts, int64(val))
	case "sqlserver.transaction_log.usage":
		mb.RecordSqlserverTransactionLogUsageDataPoint(ts, int64(val))
	case "sqlserver.transaction.rate":
		mb.RecordSqlserverTransactionRateDataPoint(ts, val)
	case "sqlserver.transaction.write.rate":
		mb.RecordSqlserverTransactionWriteRateDataPoint(ts, val)
	}
}
