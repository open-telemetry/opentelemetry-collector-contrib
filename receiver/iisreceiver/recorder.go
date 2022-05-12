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

package iisreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/iisreceiver"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/iisreceiver/internal/metadata"
)

type recordFunc = func(*metadata.MetricsBuilder, pcommon.Timestamp, float64)

type perfCounterRecorderConf struct {
	object    string
	instance  string
	recorders map[string]recordFunc
}

var perfCounterRecorders = []perfCounterRecorderConf{
	{
		object:   "Web Service",
		instance: "_Total",
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
