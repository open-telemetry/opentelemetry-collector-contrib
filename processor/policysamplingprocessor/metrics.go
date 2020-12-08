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

package policysamplingprocessor

import (
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
)

var (
	mPolicyDecisions = stats.Int64("policysampling_num_decisions", "Number of times the decisions have been made", stats.UnitDimensionless)
	mPolicyLatency   = stats.Int64("policysampling_evaluation_latency", "Duration for the evaluation of the decision", stats.UnitMilliseconds)
)

// MetricViews return the metrics views for this processor
func MetricViews() []*view.View {
	return []*view.View{
		{
			Name:        mPolicyDecisions.Name(),
			Measure:     mPolicyDecisions,
			Description: mPolicyDecisions.Description(),
			Aggregation: view.Count(),
			TagKeys: []tag.Key{
				tag.MustNewKey("decision"),
			},
		},
		{
			Name:        mPolicyLatency.Name(),
			Measure:     mPolicyLatency,
			Description: mPolicyLatency.Description(),
			TagKeys: []tag.Key{
				tag.MustNewKey("policy"),
				tag.MustNewKey("success"),
			},
			Aggregation: view.Distribution(0, 5, 10, 20, 50, 100, 200, 500, 1000, 2000, 5000),
		},
		{
			Name:        "policysampling_evaluation_outcome",
			Measure:     mPolicyLatency,
			Description: "Number of success/failures for each endpoint",
			TagKeys: []tag.Key{
				tag.MustNewKey("policy"),
				tag.MustNewKey("success"),
			},
			Aggregation: view.Count(),
		},
	}
}
