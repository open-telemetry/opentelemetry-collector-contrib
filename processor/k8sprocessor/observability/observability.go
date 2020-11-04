// Copyright 2020 OpenTelemetry Authors
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
		viewOtherUpdated,
		viewOtherAdded,
		viewOtherDeleted,
		viewIPLookupMiss,
	)
}

var (
	mPodsUpdated = stats.Int64("otelsvc/k8s/pod_updated", "Number of pod update events received", "1")
	mPodsAdded   = stats.Int64("otelsvc/k8s/pod_added", "Number of pod add events received", "1")
	mPodsDeleted = stats.Int64("otelsvc/k8s/pod_deleted", "Number of pod delete events received", "1")

	mOtherUpdated = stats.Int64("otelsvc/k8s/other_updated", "Number of other update events received", "1")
	mOtherAdded   = stats.Int64("otelsvc/k8s/other_added", "Number of other add events received", "1")
	mOtherDeleted = stats.Int64("otelsvc/k8s/other_deleted", "Number of other delete events received", "1")

	mIPLookupMiss = stats.Int64("otelsvc/k8s/ip_lookup_miss", "Number of times pod by IP lookup failed.", "1")
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

var viewOtherUpdated = &view.View{
	Name:        mOtherUpdated.Name(),
	Description: mOtherUpdated.Description(),
	Measure:     mOtherUpdated,
	Aggregation: view.Sum(),
}

var viewOtherAdded = &view.View{
	Name:        mOtherAdded.Name(),
	Description: mOtherAdded.Description(),
	Measure:     mOtherAdded,
	Aggregation: view.Sum(),
}

var viewOtherDeleted = &view.View{
	Name:        mOtherDeleted.Name(),
	Description: mOtherDeleted.Description(),
	Measure:     mOtherDeleted,
	Aggregation: view.Sum(),
}

var viewIPLookupMiss = &view.View{
	Name:        mIPLookupMiss.Name(),
	Description: mIPLookupMiss.Description(),
	Measure:     mIPLookupMiss,
	Aggregation: view.Sum(),
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

// RecordOtherUpdated increments the metric that records other update events received.
func RecordOtherUpdated() {
	stats.Record(context.Background(), mOtherUpdated.M(int64(1)))
}

// RecordOtherAdded increments the metric that records other add events receiver.
func RecordOtherAdded() {
	stats.Record(context.Background(), mOtherAdded.M(int64(1)))
}

// RecordOtherDeleted increments the metric that records other events deleted.
func RecordOtherDeleted() {
	stats.Record(context.Background(), mOtherDeleted.M(int64(1)))
}

// RecordIPLookupMiss increments the metric that records Pod lookup by IP misses.
func RecordIPLookupMiss() {
	stats.Record(context.Background(), mIPLookupMiss.M(int64(1)))
}
