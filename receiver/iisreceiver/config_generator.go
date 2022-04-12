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
	windowsapi "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/winperfcounters"
)

// getPerfCounters returns established PerfCounters for each metric.
func getScraperCfgs() []windowsapi.ObjectConfig {
	return []windowsapi.ObjectConfig{

		{
			Object:    "Web Service",
			Instances: []string{"_Total"},
			Counters: []windowsapi.CounterConfig{
				{
					MetricRep: windowsapi.MetricRep{
						Name: "iis.connection.active",
					},
					Name: "Current Connections",
				},
				{
					MetricRep: windowsapi.MetricRep{
						Name: "iis.network.io",
						Attributes: map[string]string{
							"direction": "received",
						},
					},
					Name: "Total Bytes Received",
				},
				{
					MetricRep: windowsapi.MetricRep{
						Name: "iis.network.io",
						Attributes: map[string]string{
							"direction": "sent",
						},
					},
					Name: "Total Bytes Sent",
				},
				{
					MetricRep: windowsapi.MetricRep{
						Name: "iis.connection.attempt.count",
					},
					Name: "Total Connection Attempts (all instances)",
				},
				{
					MetricRep: windowsapi.MetricRep{
						Name: "iis.request.count",
						Attributes: map[string]string{
							"request_type": "delete",
						},
					},
					Name: "Total Delete Requests",
				},
				{
					MetricRep: windowsapi.MetricRep{
						Name: "iis.request.count",
						Attributes: map[string]string{
							"request_type": "get",
						},
					},

					Name: "Total Get Requests",
				},
				{
					MetricRep: windowsapi.MetricRep{
						Name: "iis.request.count",
						Attributes: map[string]string{
							"request_type": "head",
						},
					},

					Name: "Total Head Requests",
				},
				{
					MetricRep: windowsapi.MetricRep{
						Name: "iis.request.count",
						Attributes: map[string]string{
							"request_type": "options",
						},
					},

					Name: "Total Options Requests",
				},
				{
					MetricRep: windowsapi.MetricRep{
						Name: "iis.request.count",
						Attributes: map[string]string{
							"request_type": "post",
						},
					},

					Name: "Total Post Requests",
				},
				{
					MetricRep: windowsapi.MetricRep{
						Name: "iis.request.count",
						Attributes: map[string]string{
							"request_type": "put",
						},
					},

					Name: "Total Put Requests",
				},
				{
					MetricRep: windowsapi.MetricRep{
						Name: "iis.request.count",
						Attributes: map[string]string{
							"request_type": "trace",
						},
					},

					Name: "Total Trace Requests",
				},
				{
					MetricRep: windowsapi.MetricRep{
						Name: "iis.network.file.count",
						Attributes: map[string]string{
							"direction": "received",
						},
					},
					Name: "Total Files Received",
				},
				{
					MetricRep: windowsapi.MetricRep{
						Name: "iis.network.file.count",
						Attributes: map[string]string{
							"direction": "sent",
						},
					},
					Name: "Total Files Sent",
				},
				{
					MetricRep: windowsapi.MetricRep{
						Name: "iis.connection.anonymous",
					},

					Name: "Total Anonymous Users",
				},
				{
					MetricRep: windowsapi.MetricRep{
						Name: "iis.network.blocked",
					},

					Name: "Total blocked bandwidth bytes",
				},
				{
					MetricRep: windowsapi.MetricRep{
						Name: "iis.uptime",
					},

					Name: "Service Uptime",
				},
			},
		},
		{
			Object: "HTTP Service Request Queues",
			Counters: []windowsapi.CounterConfig{
				{
					MetricRep: windowsapi.MetricRep{
						Name: "iis.request.rejected",
					},

					Name: "RejectedRequests",
				},
				{
					MetricRep: windowsapi.MetricRep{
						Name: "iis.request.queue.count",
					},

					Name: "CurrentQueueSize",
				},
				{
					MetricRep: windowsapi.MetricRep{
						Name: "iis.request.queue.age.max",
					},
					Name: "MaxQueueItemAge",
				},
			},
		},
		{
			Object:    "Process",
			Instances: []string{"_Total"},
			Counters: []windowsapi.CounterConfig{
				{
					MetricRep: windowsapi.MetricRep{
						Name: "iis.thread.active",
					},
					Name: "Thread Count",
				},
			},
		},
	}
}
