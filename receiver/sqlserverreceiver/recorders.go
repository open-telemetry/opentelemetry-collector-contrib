// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows
// +build windows

package sqlserverreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlserverreceiver"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlserverreceiver/internal/metadata"
)

const (
	// defaultObjectName is the default object name to query for metrics.
	defaultObjectName = "SQLServer"
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
		object: "General Statistics",
		recorders: map[string]recordFunc{
			"User Connections": func(mb *metadata.MetricsBuilder, ts pcommon.Timestamp, val float64) {
				mb.RecordSqlserverUserConnectionCountDataPoint(ts, int64(val))
			},
		},
	},
	{
		object: "SQL Statistics",
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
		object:   "Locks",
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
		object: "Buffer Manager",
		recorders: map[string]recordFunc{
			"Buffer cache hit ratio": func(mb *metadata.MetricsBuilder, ts pcommon.Timestamp, val float64) {
				mb.RecordSqlserverPageBufferCacheHitRatioDataPoint(ts, val)
			},
			"Checkpoint pages/sec": func(mb *metadata.MetricsBuilder, ts pcommon.Timestamp, val float64) {
				mb.RecordSqlserverPageCheckpointFlushRateDataPoint(ts, val)
			},
			"Lazy writes/sec": func(mb *metadata.MetricsBuilder, ts pcommon.Timestamp, val float64) {
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
		object:   "Access Methods",
		instance: "_Total",
		recorders: map[string]recordFunc{
			"Page Splits/sec": func(mb *metadata.MetricsBuilder, ts pcommon.Timestamp, val float64) {
				mb.RecordSqlserverPageSplitRateDataPoint(ts, val)
			},
		},
	},
	{
		object:   "Databases",
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
