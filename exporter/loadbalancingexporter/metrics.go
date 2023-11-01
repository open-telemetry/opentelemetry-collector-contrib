// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package loadbalancingexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter"

import (
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
)

var (
	mNumResolutions = stats.Int64("loadbalancer_num_resolutions", "Number of times the resolver triggered a new resolutions", stats.UnitDimensionless)
	mNumBackends    = stats.Int64("loadbalancer_num_backends", "Current number of backends in use", stats.UnitDimensionless)
	mBackendLatency = stats.Int64("loadbalancer_backend_latency", "Response latency in ms for the backends", stats.UnitMilliseconds)

	endpointTagKey      = tag.MustNewKey("endpoint")
	successTrueMutator  = tag.Upsert(tag.MustNewKey("success"), "true")
	successFalseMutator = tag.Upsert(tag.MustNewKey("success"), "false")
)

// metricViews return the metrics views according to given telemetry level.
func metricViews() []*view.View {
	return []*view.View{
		{
			Name:        mNumResolutions.Name(),
			Measure:     mNumResolutions,
			Description: mNumResolutions.Description(),
			Aggregation: view.Count(),
			TagKeys: []tag.Key{
				tag.MustNewKey("resolver"),
				tag.MustNewKey("success"),
			},
		},
		{
			Name:        mNumBackends.Name(),
			Measure:     mNumBackends,
			Description: mNumBackends.Description(),
			Aggregation: view.LastValue(),
			TagKeys: []tag.Key{
				tag.MustNewKey("resolver"),
			},
		},
		{
			Name:        "loadbalancer_num_backend_updates", // counts the number of times the measure was changed
			Measure:     mNumBackends,
			Description: "Number of times the list of backends was updated",
			Aggregation: view.Count(),
			TagKeys: []tag.Key{
				tag.MustNewKey("resolver"),
			},
		},
		{
			Name:        mBackendLatency.Name(),
			Measure:     mBackendLatency,
			Description: mBackendLatency.Description(),
			TagKeys: []tag.Key{
				tag.MustNewKey("endpoint"),
			},
			Aggregation: view.Distribution(0, 5, 10, 20, 50, 100, 200, 500, 1000, 2000, 5000),
		},
		{
			Name:        "loadbalancer_backend_outcome",
			Measure:     mBackendLatency,
			Description: "Number of success/failures for each endpoint",
			TagKeys: []tag.Key{
				tag.MustNewKey("endpoint"),
				tag.MustNewKey("success"),
			},
			Aggregation: view.Count(),
		},
	}
}
