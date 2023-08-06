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

type recordFunc = func(*metadata.ResourceMetricsBuilder, pcommon.Timestamp, float64)

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
			"User Connections": func(rmb *metadata.ResourceMetricsBuilder, ts pcommon.Timestamp, val float64) {
				rmb.RecordSqlserverUserConnectionCountDataPoint(ts, int64(val))
			},
		},
	},
	{
		object: "SQL Statistics",
		recorders: map[string]recordFunc{
			"Batch Requests/sec": func(rmb *metadata.ResourceMetricsBuilder, ts pcommon.Timestamp, val float64) {
				rmb.RecordSqlserverBatchRequestRateDataPoint(ts, val)
			},
			"SQL Compilations/sec": func(rmb *metadata.ResourceMetricsBuilder, ts pcommon.Timestamp, val float64) {
				rmb.RecordSqlserverBatchSQLCompilationRateDataPoint(ts, val)
			},
			"SQL Re-Compilations/sec": func(rmb *metadata.ResourceMetricsBuilder, ts pcommon.Timestamp, val float64) {
				rmb.RecordSqlserverBatchSQLRecompilationRateDataPoint(ts, val)
			},
		},
	},
	{
		object:   "Locks",
		instance: "_Total",
		recorders: map[string]recordFunc{
			"Lock Waits/sec": func(rmb *metadata.ResourceMetricsBuilder, ts pcommon.Timestamp, val float64) {
				rmb.RecordSqlserverLockWaitRateDataPoint(ts, val)
			},
			"Average Wait Time (ms)": func(rmb *metadata.ResourceMetricsBuilder, ts pcommon.Timestamp, val float64) {
				rmb.RecordSqlserverLockWaitTimeAvgDataPoint(ts, val)
			},
		},
	},
	{
		object: "Buffer Manager",
		recorders: map[string]recordFunc{
			"Buffer cache hit ratio": func(rmb *metadata.ResourceMetricsBuilder, ts pcommon.Timestamp, val float64) {
				rmb.RecordSqlserverPageBufferCacheHitRatioDataPoint(ts, val)
			},
			"Checkpoint pages/sec": func(rmb *metadata.ResourceMetricsBuilder, ts pcommon.Timestamp, val float64) {
				rmb.RecordSqlserverPageCheckpointFlushRateDataPoint(ts, val)
			},
			"Lazy writes/sec": func(rmb *metadata.ResourceMetricsBuilder, ts pcommon.Timestamp, val float64) {
				rmb.RecordSqlserverPageLazyWriteRateDataPoint(ts, val)
			},
			"Page life expectancy": func(rmb *metadata.ResourceMetricsBuilder, ts pcommon.Timestamp, val float64) {
				rmb.RecordSqlserverPageLifeExpectancyDataPoint(ts, int64(val))
			},
			"Page reads/sec": func(rmb *metadata.ResourceMetricsBuilder, ts pcommon.Timestamp, val float64) {
				rmb.RecordSqlserverPageOperationRateDataPoint(ts, val, metadata.AttributePageOperationsRead)
			},
			"Page writes/sec": func(rmb *metadata.ResourceMetricsBuilder, ts pcommon.Timestamp, val float64) {
				rmb.RecordSqlserverPageOperationRateDataPoint(ts, val, metadata.AttributePageOperationsWrite)
			},
		},
	},
	{
		object:   "Access Methods",
		instance: "_Total",
		recorders: map[string]recordFunc{
			"Page Splits/sec": func(rmb *metadata.ResourceMetricsBuilder, ts pcommon.Timestamp, val float64) {
				rmb.RecordSqlserverPageSplitRateDataPoint(ts, val)
			},
		},
	},
	{
		object:   "Databases",
		instance: "*",
		recorders: map[string]recordFunc{
			"Log Bytes Flushed/sec": func(rmb *metadata.ResourceMetricsBuilder, ts pcommon.Timestamp, val float64) {
				rmb.RecordSqlserverTransactionLogFlushDataRateDataPoint(ts, val)
			},
			"Log Flushes/sec": func(rmb *metadata.ResourceMetricsBuilder, ts pcommon.Timestamp, val float64) {
				rmb.RecordSqlserverTransactionLogFlushRateDataPoint(ts, val)
			},
			"Log Flush Waits/sec": func(rmb *metadata.ResourceMetricsBuilder, ts pcommon.Timestamp, val float64) {
				rmb.RecordSqlserverTransactionLogFlushWaitRateDataPoint(ts, val)
			},
			"Log Growths": func(rmb *metadata.ResourceMetricsBuilder, ts pcommon.Timestamp, val float64) {
				rmb.RecordSqlserverTransactionLogGrowthCountDataPoint(ts, int64(val))
			},
			"Log Shrinks": func(rmb *metadata.ResourceMetricsBuilder, ts pcommon.Timestamp, val float64) {
				rmb.RecordSqlserverTransactionLogShrinkCountDataPoint(ts, int64(val))
			},
			"Percent Log Used": func(rmb *metadata.ResourceMetricsBuilder, ts pcommon.Timestamp, val float64) {
				rmb.RecordSqlserverTransactionLogUsageDataPoint(ts, int64(val))
			},
			"Transactions/sec": func(rmb *metadata.ResourceMetricsBuilder, ts pcommon.Timestamp, val float64) {
				rmb.RecordSqlserverTransactionRateDataPoint(ts, val)
			},
			"Write Transactions/sec": func(rmb *metadata.ResourceMetricsBuilder, ts pcommon.Timestamp, val float64) {
				rmb.RecordSqlserverTransactionWriteRateDataPoint(ts, val)
			},
		},
	},
}
