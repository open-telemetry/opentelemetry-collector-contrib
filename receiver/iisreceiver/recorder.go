// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows
// +build windows

package iisreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/iisreceiver"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/iisreceiver/internal/metadata"
)

type recordFunc = func(md *metadata.ResourceMetricsBuilder, ts pcommon.Timestamp, val float64)

type perfCounterRecorderConf struct {
	object    string
	instance  string
	recorders map[string]recordFunc
}

var totalPerfCounterRecorders = []perfCounterRecorderConf{
	{
		object:   "Process",
		instance: "_Total",
		recorders: map[string]recordFunc{
			"Thread Count": func(rmb *metadata.ResourceMetricsBuilder, ts pcommon.Timestamp, val float64) {
				rmb.RecordIisThreadActiveDataPoint(ts, int64(val))
			},
		},
	},
}

var sitePerfCounterRecorders = []perfCounterRecorderConf{
	{
		object:   "Web Service",
		instance: "*",
		recorders: map[string]recordFunc{
			"Current Connections": func(rmb *metadata.ResourceMetricsBuilder, ts pcommon.Timestamp, val float64) {
				rmb.RecordIisConnectionActiveDataPoint(ts, int64(val))
			},
			"Total Bytes Received": func(rmb *metadata.ResourceMetricsBuilder, ts pcommon.Timestamp, val float64) {
				rmb.RecordIisNetworkIoDataPoint(ts, int64(val), metadata.AttributeDirectionReceived)
			},
			"Total Bytes Sent": func(rmb *metadata.ResourceMetricsBuilder, ts pcommon.Timestamp, val float64) {
				rmb.RecordIisNetworkIoDataPoint(ts, int64(val), metadata.AttributeDirectionSent)
			},
			"Total Connection Attempts (all instances)": func(rmb *metadata.ResourceMetricsBuilder,
				ts pcommon.Timestamp,
				val float64) {
				rmb.RecordIisConnectionAttemptCountDataPoint(ts, int64(val))
			},
			"Total Delete Requests": func(rmb *metadata.ResourceMetricsBuilder, ts pcommon.Timestamp, val float64) {
				rmb.RecordIisRequestCountDataPoint(ts, int64(val), metadata.AttributeRequestDelete)
			},
			"Total Get Requests": func(rmb *metadata.ResourceMetricsBuilder, ts pcommon.Timestamp, val float64) {
				rmb.RecordIisRequestCountDataPoint(ts, int64(val), metadata.AttributeRequestGet)
			},
			"Total Head Requests": func(rmb *metadata.ResourceMetricsBuilder, ts pcommon.Timestamp, val float64) {
				rmb.RecordIisRequestCountDataPoint(ts, int64(val), metadata.AttributeRequestHead)
			},
			"Total Options Requests": func(rmb *metadata.ResourceMetricsBuilder, ts pcommon.Timestamp, val float64) {
				rmb.RecordIisRequestCountDataPoint(ts, int64(val), metadata.AttributeRequestOptions)
			},
			"Total Post Requests": func(rmb *metadata.ResourceMetricsBuilder, ts pcommon.Timestamp, val float64) {
				rmb.RecordIisRequestCountDataPoint(ts, int64(val), metadata.AttributeRequestPost)
			},
			"Total Put Requests": func(rmb *metadata.ResourceMetricsBuilder, ts pcommon.Timestamp, val float64) {
				rmb.RecordIisRequestCountDataPoint(ts, int64(val), metadata.AttributeRequestPut)
			},
			"Total Trace Requests": func(rmb *metadata.ResourceMetricsBuilder, ts pcommon.Timestamp, val float64) {
				rmb.RecordIisRequestCountDataPoint(ts, int64(val), metadata.AttributeRequestTrace)
			},
			"Total Files Received": func(rmb *metadata.ResourceMetricsBuilder, ts pcommon.Timestamp, val float64) {
				rmb.RecordIisNetworkFileCountDataPoint(ts, int64(val), metadata.AttributeDirectionReceived)
			},
			"Total Files Sent": func(rmb *metadata.ResourceMetricsBuilder, ts pcommon.Timestamp, val float64) {
				rmb.RecordIisNetworkFileCountDataPoint(ts, int64(val), metadata.AttributeDirectionSent)
			},
			"Total Anonymous Users": func(rmb *metadata.ResourceMetricsBuilder, ts pcommon.Timestamp, val float64) {
				rmb.RecordIisConnectionAnonymousDataPoint(ts, int64(val))
			},
			"Total blocked bandwidth bytes.": func(rmb *metadata.ResourceMetricsBuilder, ts pcommon.Timestamp,
				val float64) {
				rmb.RecordIisNetworkBlockedDataPoint(ts, int64(val))
			},
			"Service Uptime": func(rmb *metadata.ResourceMetricsBuilder, ts pcommon.Timestamp, val float64) {
				rmb.RecordIisUptimeDataPoint(ts, int64(val))
			},
		},
	},
}

var appPoolPerfCounterRecorders = []perfCounterRecorderConf{
	{
		object:   "HTTP Service Request Queues",
		instance: "*",
		recorders: map[string]recordFunc{
			"RejectedRequests": func(rmb *metadata.ResourceMetricsBuilder, ts pcommon.Timestamp, val float64) {
				rmb.RecordIisRequestRejectedDataPoint(ts, int64(val))
			},
			"CurrentQueueSize": func(rmb *metadata.ResourceMetricsBuilder, ts pcommon.Timestamp, val float64) {
				rmb.RecordIisRequestQueueCountDataPoint(ts, int64(val))
			},
		},
	},
}

