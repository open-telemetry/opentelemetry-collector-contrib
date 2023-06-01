// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows
// +build windows

package iisreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/iisreceiver"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/iisreceiver/internal/metadata"
)

type recordFunc = func(md *metadata.MetricsBuilder, ts pcommon.Timestamp, val float64)

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
			"Thread Count": func(mb *metadata.MetricsBuilder, ts pcommon.Timestamp, val float64) {
				mb.RecordIisThreadActiveDataPoint(ts, int64(val))
			},
		},
	},
}

var sitePerfCounterRecorders = []perfCounterRecorderConf{
	{
		object:   "Web Service",
		instance: "*",
		recorders: map[string]recordFunc{
			"Current Connections": func(mb *metadata.MetricsBuilder, ts pcommon.Timestamp, val float64) {
				mb.RecordIisConnectionActiveDataPoint(ts, int64(val))
			},
			"Total Bytes Received": func(mb *metadata.MetricsBuilder, ts pcommon.Timestamp, val float64) {
				mb.RecordIisNetworkIoDataPoint(ts, int64(val), metadata.AttributeDirectionReceived)
			},
			"Total Bytes Sent": func(mb *metadata.MetricsBuilder, ts pcommon.Timestamp, val float64) {
				mb.RecordIisNetworkIoDataPoint(ts, int64(val), metadata.AttributeDirectionSent)
			},
			"Total Connection Attempts (all instances)": func(mb *metadata.MetricsBuilder, ts pcommon.Timestamp,
				val float64) {
				mb.RecordIisConnectionAttemptCountDataPoint(ts, int64(val))
			},
			"Total Delete Requests": func(mb *metadata.MetricsBuilder, ts pcommon.Timestamp, val float64) {
				mb.RecordIisRequestCountDataPoint(ts, int64(val), metadata.AttributeRequestDelete)
			},
			"Total Get Requests": func(mb *metadata.MetricsBuilder, ts pcommon.Timestamp, val float64) {
				mb.RecordIisRequestCountDataPoint(ts, int64(val), metadata.AttributeRequestGet)
			},
			"Total Head Requests": func(mb *metadata.MetricsBuilder, ts pcommon.Timestamp, val float64) {
				mb.RecordIisRequestCountDataPoint(ts, int64(val), metadata.AttributeRequestHead)
			},
			"Total Options Requests": func(mb *metadata.MetricsBuilder, ts pcommon.Timestamp, val float64) {
				mb.RecordIisRequestCountDataPoint(ts, int64(val), metadata.AttributeRequestOptions)
			},
			"Total Post Requests": func(mb *metadata.MetricsBuilder, ts pcommon.Timestamp, val float64) {
				mb.RecordIisRequestCountDataPoint(ts, int64(val), metadata.AttributeRequestPost)
			},
			"Total Put Requests": func(mb *metadata.MetricsBuilder, ts pcommon.Timestamp, val float64) {
				mb.RecordIisRequestCountDataPoint(ts, int64(val), metadata.AttributeRequestPut)
			},
			"Total Trace Requests": func(mb *metadata.MetricsBuilder, ts pcommon.Timestamp, val float64) {
				mb.RecordIisRequestCountDataPoint(ts, int64(val), metadata.AttributeRequestTrace)
			},
			"Total Files Received": func(mb *metadata.MetricsBuilder, ts pcommon.Timestamp, val float64) {
				mb.RecordIisNetworkFileCountDataPoint(ts, int64(val), metadata.AttributeDirectionReceived)
			},
			"Total Files Sent": func(mb *metadata.MetricsBuilder, ts pcommon.Timestamp, val float64) {
				mb.RecordIisNetworkFileCountDataPoint(ts, int64(val), metadata.AttributeDirectionSent)
			},
			"Total Anonymous Users": func(mb *metadata.MetricsBuilder, ts pcommon.Timestamp, val float64) {
				mb.RecordIisConnectionAnonymousDataPoint(ts, int64(val))
			},
			"Total blocked bandwidth bytes.": func(mb *metadata.MetricsBuilder, ts pcommon.Timestamp, val float64) {
				mb.RecordIisNetworkBlockedDataPoint(ts, int64(val))
			},
			"Service Uptime": func(mb *metadata.MetricsBuilder, ts pcommon.Timestamp, val float64) {
				mb.RecordIisUptimeDataPoint(ts, int64(val))
			},
		},
	},
}

var appPoolPerfCounterRecorders = []perfCounterRecorderConf{
	{
		object:   "HTTP Service Request Queues",
		instance: "*",
		recorders: map[string]recordFunc{
			"RejectedRequests": func(mb *metadata.MetricsBuilder, ts pcommon.Timestamp, val float64) {
				mb.RecordIisRequestRejectedDataPoint(ts, int64(val))
			},
			"CurrentQueueSize": func(mb *metadata.MetricsBuilder, ts pcommon.Timestamp, val float64) {
				mb.RecordIisRequestQueueCountDataPoint(ts, int64(val))
			},
			"MaxQueueItemAge": func(mb *metadata.MetricsBuilder, ts pcommon.Timestamp, val float64) {
				mb.RecordIisRequestQueueAgeMaxDataPoint(ts, int64(val))
			},
		},
	},
}
