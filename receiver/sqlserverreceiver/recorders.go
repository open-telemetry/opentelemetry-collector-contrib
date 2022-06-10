// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//go:build windows
// +build windows

package sqlserverreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlserverreceiver"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlserverreceiver/internal/metadata"
)

type recordFunc = func(*metadata.MetricsBuilder, pcommon.Timestamp, float64)

type perfCounterRecorderConf struct {
	object    string
	instance  string
	recorders map[string]recordFunc
}

// perfCounterRecorders is map of perf counter object -> perf counter name -> value recorder.
var perfCounterRecorders = []perfCounterRecorderConf{
	{
		object: "SQLServer:General Statistics",
		recorders: map[string]recordFunc{
			"User Connections": func(mb *metadata.MetricsBuilder, ts pcommon.Timestamp, val float64) {
				mb.RecordSqlserverUserConnectionCountDataPoint(ts, int64(val))
			},
		},
	},
	{
		object: "SQLServer:SQL Statistics",
		recorders: map[string]recordFunc{
			"Batch Requests/sec": func(mb *metadata.MetricsBuilder, ts pcommon.Timestamp, val float64) {
				mb.RecordSqlserverBatchRequestRateDataPoint(ts, val)
			},
			"SQL Compilations/sec": func(mb *metadata.MetricsBuilder, ts pcommon.Timestamp, val float64) {
				mb.RecordSqlserverBatchSQLCompilationRateDataPoint(ts, val)
			},
			"SQL Re-Compilations/sec": func(mb *metadata.MetricsBuilder, ts pcommon.Timestamp, val float64) {
				mb.RecordSqlserverBatchSQLRecompilationRateDataPoint(ts, val)
			},
		},
	},
	{
		object:   "SQLServer:Locks",
		instance: "_Total",
		recorders: map[string]recordFunc{
			"Lock Waits/sec": func(mb *metadata.MetricsBuilder, ts pcommon.Timestamp, val float64) {
				mb.RecordSqlserverLockWaitRateDataPoint(ts, val)
			},
			"Average Wait Time (ms)": func(mb *metadata.MetricsBuilder, ts pcommon.Timestamp, val float64) {
				mb.RecordSqlserverLockWaitTimeAvgDataPoint(ts, val)
			},
		},
	},
	{
		object: "SQLServer:Buffer Manager",
		recorders: map[string]recordFunc{
			"Buffer cache hit ratio": func(mb *metadata.MetricsBuilder, ts pcommon.Timestamp, val float64) {
				mb.RecordSqlserverPageBufferCacheHitRatioDataPoint(ts, val)
			},
			"Checkpoint pages/sec": func(mb *metadata.MetricsBuilder, ts pcommon.Timestamp, val float64) {
				mb.RecordSqlserverPageCheckpointFlushRateDataPoint(ts, val)
			},
			"Lazy Writes/sec": func(mb *metadata.MetricsBuilder, ts pcommon.Timestamp, val float64) {
				mb.RecordSqlserverPageLazyWriteRateDataPoint(ts, val)
			},
			"Page life expectancy": func(mb *metadata.MetricsBuilder, ts pcommon.Timestamp, val float64) {
				mb.RecordSqlserverPageLifeExpectancyDataPoint(ts, int64(val))
			},
			"Page reads/sec": func(mb *metadata.MetricsBuilder, ts pcommon.Timestamp, val float64) {
				mb.RecordSqlserverPageOperationRateDataPoint(ts, val, metadata.AttributePageOperationsRead)
			},
			"Page writes/sec": func(mb *metadata.MetricsBuilder, ts pcommon.Timestamp, val float64) {
				mb.RecordSqlserverPageOperationRateDataPoint(ts, val, metadata.AttributePageOperationsWrite)
			},
		},
	},
	{
		object:   "SQLServer:Access Methods",
		instance: "_Total",
		recorders: map[string]recordFunc{
			"Page Splits/sec": func(mb *metadata.MetricsBuilder, ts pcommon.Timestamp, val float64) {
				mb.RecordSqlserverPageSplitRateDataPoint(ts, val)
			},
		},
	},
	{
		object:   "SQLServer:Databases",
		instance: "*",
		recorders: map[string]recordFunc{
			"Log Bytes Flushed/sec": func(mb *metadata.MetricsBuilder, ts pcommon.Timestamp, val float64) {
				mb.RecordSqlserverTransactionLogFlushDataRateDataPoint(ts, val)
			},
			"Log Flushes/sec": func(mb *metadata.MetricsBuilder, ts pcommon.Timestamp, val float64) {
				mb.RecordSqlserverTransactionLogFlushRateDataPoint(ts, val)
			},
			"Log Flush Waits/sec": func(mb *metadata.MetricsBuilder, ts pcommon.Timestamp, val float64) {
				mb.RecordSqlserverTransactionLogFlushWaitRateDataPoint(ts, val)
			},
			"Log Growths": func(mb *metadata.MetricsBuilder, ts pcommon.Timestamp, val float64) {
				mb.RecordSqlserverTransactionLogGrowthCountDataPoint(ts, int64(val))
			},
			"Log Shrinks": func(mb *metadata.MetricsBuilder, ts pcommon.Timestamp, val float64) {
				mb.RecordSqlserverTransactionLogShrinkCountDataPoint(ts, int64(val))
			},
			"Percent Log Used": func(mb *metadata.MetricsBuilder, ts pcommon.Timestamp, val float64) {
				mb.RecordSqlserverTransactionLogUsageDataPoint(ts, int64(val))
			},
			"Transactions/sec": func(mb *metadata.MetricsBuilder, ts pcommon.Timestamp, val float64) {
				mb.RecordSqlserverTransactionRateDataPoint(ts, val)
			},
			"Write Transactions/sec": func(mb *metadata.MetricsBuilder, ts pcommon.Timestamp, val float64) {
				mb.RecordSqlserverTransactionWriteRateDataPoint(ts, val)
			},
		},
	},
}
