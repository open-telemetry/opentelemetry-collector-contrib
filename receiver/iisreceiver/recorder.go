// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows

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

func buildTotalPerfCounterRecordersFromConfig(metricsConfig metadata.MetricsConfig) []perfCounterRecorderConf {
	if !metricsConfig.IisThreadActive.Enabled {
		// if the metric is not enabled, we don't need to build any recorders
		return nil
	}

	return []perfCounterRecorderConf{
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
}

func buildSitePerfCounterRecordersFromConfig(metricsConfig metadata.MetricsConfig) []perfCounterRecorderConf {
	var sitePerfCounterRecorders []perfCounterRecorderConf

	if metricsConfig.IisConnectionActive.Enabled ||
		metricsConfig.IisNetworkIo.Enabled ||
		metricsConfig.IisConnectionAttemptCount.Enabled ||
		metricsConfig.IisRequestCount.Enabled ||
		metricsConfig.IisNetworkFileCount.Enabled ||
		metricsConfig.IisConnectionAnonymous.Enabled ||
		metricsConfig.IisNetworkBlocked.Enabled ||
		metricsConfig.IisUptime.Enabled {
		siteRecorders := map[string]recordFunc{}

		if metricsConfig.IisConnectionActive.Enabled {
			siteRecorders["Current Connections"] = func(mb *metadata.MetricsBuilder, ts pcommon.Timestamp, val float64) {
				mb.RecordIisConnectionActiveDataPoint(ts, int64(val))
			}
		}

		if metricsConfig.IisNetworkIo.Enabled {
			siteRecorders["Total Bytes Received"] = func(mb *metadata.MetricsBuilder, ts pcommon.Timestamp, val float64) {
				mb.RecordIisNetworkIoDataPoint(ts, int64(val), metadata.AttributeDirectionReceived)
			}
			siteRecorders["Total Bytes Sent"] = func(mb *metadata.MetricsBuilder, ts pcommon.Timestamp, val float64) {
				mb.RecordIisNetworkIoDataPoint(ts, int64(val), metadata.AttributeDirectionSent)
			}
		}

		if metricsConfig.IisConnectionAttemptCount.Enabled {
			siteRecorders["Total Connection Attempts (all instances)"] = func(mb *metadata.MetricsBuilder, ts pcommon.Timestamp,
				val float64,
			) {
				mb.RecordIisConnectionAttemptCountDataPoint(ts, int64(val))
			}
		}

		if metricsConfig.IisRequestCount.Enabled {
			siteRecorders["Total Delete Requests"] = func(mb *metadata.MetricsBuilder, ts pcommon.Timestamp, val float64) {
				mb.RecordIisRequestCountDataPoint(ts, int64(val), metadata.AttributeRequestDelete)
			}
			siteRecorders["Total Get Requests"] = func(mb *metadata.MetricsBuilder, ts pcommon.Timestamp, val float64) {
				mb.RecordIisRequestCountDataPoint(ts, int64(val), metadata.AttributeRequestGet)
			}
			siteRecorders["Total Head Requests"] = func(mb *metadata.MetricsBuilder, ts pcommon.Timestamp, val float64) {
				mb.RecordIisRequestCountDataPoint(ts, int64(val), metadata.AttributeRequestHead)
			}
			siteRecorders["Total Options Requests"] = func(mb *metadata.MetricsBuilder, ts pcommon.Timestamp, val float64) {
				mb.RecordIisRequestCountDataPoint(ts, int64(val), metadata.AttributeRequestOptions)
			}
			siteRecorders["Total Post Requests"] = func(mb *metadata.MetricsBuilder, ts pcommon.Timestamp, val float64) {
				mb.RecordIisRequestCountDataPoint(ts, int64(val), metadata.AttributeRequestPost)
			}
			siteRecorders["Total Put Requests"] = func(mb *metadata.MetricsBuilder, ts pcommon.Timestamp, val float64) {
				mb.RecordIisRequestCountDataPoint(ts, int64(val), metadata.AttributeRequestPut)
			}
			siteRecorders["Total Trace Requests"] = func(mb *metadata.MetricsBuilder, ts pcommon.Timestamp, val float64) {
				mb.RecordIisRequestCountDataPoint(ts, int64(val), metadata.AttributeRequestTrace)
			}
		}

		if metricsConfig.IisNetworkFileCount.Enabled {
			siteRecorders["Total Files Received"] = func(mb *metadata.MetricsBuilder, ts pcommon.Timestamp, val float64) {
				mb.RecordIisNetworkFileCountDataPoint(ts, int64(val), metadata.AttributeDirectionReceived)
			}
			siteRecorders["Total Files Sent"] = func(mb *metadata.MetricsBuilder, ts pcommon.Timestamp, val float64) {
				mb.RecordIisNetworkFileCountDataPoint(ts, int64(val), metadata.AttributeDirectionSent)
			}
		}

		if metricsConfig.IisConnectionAnonymous.Enabled {
			siteRecorders["Total Anonymous Users"] = func(mb *metadata.MetricsBuilder, ts pcommon.Timestamp, val float64) {
				mb.RecordIisConnectionAnonymousDataPoint(ts, int64(val))
			}
		}

		if metricsConfig.IisNetworkBlocked.Enabled {
			siteRecorders["Total blocked bandwidth bytes."] = func(mb *metadata.MetricsBuilder, ts pcommon.Timestamp, val float64) {
				mb.RecordIisNetworkBlockedDataPoint(ts, int64(val))
			}
		}

		if metricsConfig.IisUptime.Enabled {
			siteRecorders["Service Uptime"] = func(mb *metadata.MetricsBuilder, ts pcommon.Timestamp, val float64) {
				mb.RecordIisUptimeDataPoint(ts, int64(val))
			}
		}

		sitePerfCounterRecorders = append(sitePerfCounterRecorders, perfCounterRecorderConf{
			object:    "Web Service",
			instance:  "*",
			recorders: siteRecorders,
		})
	}

	return sitePerfCounterRecorders
}

func buildAppPoolPerfCounterRecordersFromConfig(metricsConfig metadata.MetricsConfig) []perfCounterRecorderConf {
	appPoolPerfCounterRecorders := []perfCounterRecorderConf{}

	if metricsConfig.IisRequestRejected.Enabled || metricsConfig.IisRequestQueueCount.Enabled {
		requestRecorders := map[string]recordFunc{}
		if metricsConfig.IisRequestRejected.Enabled {
			requestRecorders["RejectedRequests"] = func(mb *metadata.MetricsBuilder, ts pcommon.Timestamp, val float64) {
				mb.RecordIisRequestRejectedDataPoint(ts, int64(val))
			}
		}
		if metricsConfig.IisRequestQueueCount.Enabled {
			requestRecorders["CurrentQueueSize"] = func(mb *metadata.MetricsBuilder, ts pcommon.Timestamp, val float64) {
				mb.RecordIisRequestQueueCountDataPoint(ts, int64(val))
			}
		}
		appPoolPerfCounterRecorders = append(appPoolPerfCounterRecorders, perfCounterRecorderConf{
			object:    "HTTP Service Request Queues",
			instance:  "*",
			recorders: requestRecorders,
		})
	}

	if metricsConfig.IisApplicationPoolState.Enabled || metricsConfig.IisApplicationPoolUptime.Enabled {
		appPoolRecorders := map[string]recordFunc{}
		if metricsConfig.IisApplicationPoolState.Enabled {
			appPoolRecorders["Current Application Pool State"] = func(mb *metadata.MetricsBuilder, ts pcommon.Timestamp, val float64) {
				mb.RecordIisApplicationPoolStateDataPoint(ts, int64(val))
			}
		}
		if metricsConfig.IisApplicationPoolUptime.Enabled {
			appPoolRecorders["Current Application Pool Uptime"] = func(mb *metadata.MetricsBuilder, ts pcommon.Timestamp, val float64) {
				mb.RecordIisApplicationPoolUptimeDataPoint(ts, int64(val))
			}
		}
		appPoolPerfCounterRecorders = append(appPoolPerfCounterRecorders, perfCounterRecorderConf{
			object:    "APP_POOL_WAS",
			instance:  "*",
			recorders: appPoolRecorders,
		})
	}

	return appPoolPerfCounterRecorders
}

func recordMaxQueueItemAge(mb *metadata.MetricsBuilder, ts pcommon.Timestamp, val float64) {
	mb.RecordIisRequestQueueAgeMaxDataPoint(ts, int64(val))
}
