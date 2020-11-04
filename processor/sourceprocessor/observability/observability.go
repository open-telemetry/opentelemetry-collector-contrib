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

	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
)

func init() {
	view.Register(
		viewResourceSpansProcessed,
		viewRecordsFilteredOut,
		viewRecordsFilteredIn,
	)
}

var (
	mResouceSpansProcessed = stats.Int64("otelsvc/sumo/resource_spans_processed", "Number of record span packages processed", "1")
	mRecordsFilteredOut    = stats.Int64("otelsvc/sumo/records_filtered_out", "Number of records filtered out", "1")
	mRecordsFilteredIn     = stats.Int64("otelsvc/sumo/records_filtered_in", "Number of records filtered in", "1")
)

var viewResourceSpansProcessed = &view.View{
	Name:        mResouceSpansProcessed.Name(),
	Description: mResouceSpansProcessed.Description(),
	Measure:     mResouceSpansProcessed,
	Aggregation: view.Sum(),
}

var viewRecordsFilteredOut = &view.View{
	Name:        mRecordsFilteredOut.Name(),
	Description: mRecordsFilteredOut.Description(),
	Measure:     mRecordsFilteredOut,
	Aggregation: view.Sum(),
}

var viewRecordsFilteredIn = &view.View{
	Name:        mRecordsFilteredIn.Name(),
	Description: mRecordsFilteredIn.Description(),
	Measure:     mRecordsFilteredIn,
	Aggregation: view.Sum(),
}

// RecordResourceSpansProcessed increments the metric that resource spans package was processed
func RecordResourceSpansProcessed() {
	stats.Record(context.Background(), mResouceSpansProcessed.M(int64(1)))
}

// RecordFilteredOut increments the metric that records record filtered out
func RecordFilteredOut() {
	stats.Record(context.Background(), mRecordsFilteredOut.M(int64(1)))
}

// RecordFilteredOutN increments the metric that records record filtered out
func RecordFilteredOutN(n int) {
	stats.Record(context.Background(), mRecordsFilteredOut.M(int64(n)))
}

// RecordFilteredIn increments the metric that records record filtered in
func RecordFilteredIn() {
	stats.Record(context.Background(), mRecordsFilteredIn.M(int64(1)))
}

// RecordFilteredInN increments the metric that records record filtered in
func RecordFilteredInN(n int) {
	stats.Record(context.Background(), mRecordsFilteredIn.M(int64(n)))
}
