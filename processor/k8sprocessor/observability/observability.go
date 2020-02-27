// Copyright 2019 Omnition Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package observability

import (
	"context"
	"time"

	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
)

// TODO: re-think if processor should register it's own telemetry views or if some other
// mechanism should be used by the collector to discover views from all components

func init() {
	view.Register(
		viewPodsUpdated,
		viewPodsAdded,
		viewPodsDeleted,
		viewIPLookupMiss,
		viewAPICallMade,
		viewAPICallLatency,
	)
}

var (
	mPodsUpdated = stats.Int64("otelsvc/k8s/pod_updated", "Number of pod update events received", "1")
	mPodsAdded   = stats.Int64("otelsvc/k8s/pod_added", "Number of pod add events received", "1")
	mPodsDeleted = stats.Int64("otelsvc/k8s/pod_deleted", "Number of pod delete events received", "1")

	mIPLookupMiss = stats.Int64("otelsvc/k8s/ip_lookup_miss", "Number of times pod by IP lookup failed.", "1")

	mAPICallMade    = stats.Int64("otelsvc/k8s/api_call_made", "Number of times K8S API calls were made", "1")
	mAPICallLatency = stats.Float64("otelsvc/k8s/api_call_latency", "The latency in milliseconds per K8S API call", "ms")
)

var viewPodsUpdated = &view.View{
	Name:        mPodsUpdated.Name(),
	Description: mPodsUpdated.Description(),
	Measure:     mPodsUpdated,
	Aggregation: view.Sum(),
}

var viewPodsAdded = &view.View{
	Name:        mPodsAdded.Name(),
	Description: mPodsAdded.Description(),
	Measure:     mPodsAdded,
	Aggregation: view.Sum(),
}

var viewPodsDeleted = &view.View{
	Name:        mPodsDeleted.Name(),
	Description: mPodsDeleted.Description(),
	Measure:     mPodsDeleted,
	Aggregation: view.Sum(),
}

var viewIPLookupMiss = &view.View{
	Name:        mIPLookupMiss.Name(),
	Description: mIPLookupMiss.Description(),
	Measure:     mIPLookupMiss,
	Aggregation: view.Sum(),
}

var viewAPICallMade = &view.View{
	Name:        mAPICallMade.Name(),
	Description: mAPICallMade.Description(),
	Measure:     mAPICallMade,
	Aggregation: view.Sum(),
}

var viewAPICallLatency = &view.View{
	Name:        mAPICallLatency.Name(),
	Description: mAPICallLatency.Description(),
	Measure:     mAPICallLatency,
	Aggregation: view.Distribution(0, 5, 10, 25, 50, 100, 250, 500, 1000, 2500, 5000, 10000),
}

// RecordPodUpdated increments the metric that records pod update events received.
func RecordPodUpdated() {
	stats.Record(context.Background(), mPodsUpdated.M(int64(1)))
}

// RecordPodAdded increments the metric that records pod add events receiver.
func RecordPodAdded() {
	stats.Record(context.Background(), mPodsAdded.M(int64(1)))
}

// RecordPodDeleted increments the metric that records pod events deleted.
func RecordPodDeleted() {
	stats.Record(context.Background(), mPodsDeleted.M(int64(1)))
}

// RecordIPLookupMiss increments the metric that records Pod lookup by IP misses.
func RecordIPLookupMiss() {
	stats.Record(context.Background(), mIPLookupMiss.M(int64(1)))
}

// RecordAPICallMadeAndLatency increments the metrics that records K8S lookups and measures their duration
func RecordAPICallMadeAndLatency(startTime *time.Time) {
	stats.Record(context.Background(), mAPICallMade.M(int64(1)))
	stats.Record(context.Background(), mAPICallLatency.M(sinceInMilliseconds(startTime)))
}

func sinceInMilliseconds(startTime *time.Time) float64 {
	return float64(time.Since(*startTime).Nanoseconds()) / 1e6
}
