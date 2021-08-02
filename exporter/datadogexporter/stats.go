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

package datadogexporter

import (
	"time"

	"github.com/DataDog/datadog-agent/pkg/trace/exportable/pb"
	"github.com/DataDog/datadog-agent/pkg/trace/exportable/stats"
)

const (
	statsBucketDuration   int64  = int64(10 * time.Second)
	versionAggregationTag string = "version"
)

// ComputeAPMStats calculates the stats that should be submitted to APM about a given trace
func computeAPMStats(tracePayload *pb.TracePayload, pushTime int64) *stats.Payload {
	statsRawBuckets := make(map[int64]*stats.RawBucket)

	// removing sublayer calc as part of work to port
	// https://github.com/DataDog/datadog-agent/pull/7450/files
	var emptySublayer []stats.SublayerValue

	bucketTS := pushTime - statsBucketDuration
	for _, trace := range tracePayload.Traces {
		spans := getAnalyzedSpans(trace.Spans)

		for _, span := range spans {

			// TODO: While this is hardcoded to assume a single 10s buckets for now,
			// An improvement would be to support keeping multiple 10s buckets in buffer
			// ala, [0-10][10-20][20-30], only flushing the oldest bucket, to allow traces that
			// get reported late to still be counted in the correct bucket. This is how the
			// datadog- agent handles stats buckets, but would be non trivial to add.

			var statsRawBucket *stats.RawBucket
			if existingBucket, ok := statsRawBuckets[bucketTS]; ok {
				statsRawBucket = existingBucket
			} else {
				statsRawBucket = stats.NewRawBucket(bucketTS, statsBucketDuration)
				statsRawBuckets[bucketTS] = statsRawBucket
			}

			// Use weight 1, as sampling in opentelemetry would occur upstream in a processor.
			// Generally we want to ship 100% of traces to the backend where more accurate tail based sampling can be performed.
			// TopLevel is always "true" since we only compute stats for top-level spans.

			var spanWeight float64
			if spanRate, ok := span.Metrics[keySamplingRate]; ok {
				spanWeight = (1 / spanRate)
			} else {
				spanWeight = 1
			}

			weightedSpan := &stats.WeightedSpan{
				Span:     span,
				Weight:   spanWeight,
				TopLevel: true,
			}
			statsRawBucket.HandleSpan(weightedSpan, tracePayload.Env, []string{versionAggregationTag}, emptySublayer)
		}
	}

	// Export statsRawBuckets to statsBuckets
	statsBuckets := make([]stats.Bucket, 0)
	for _, statsRawBucket := range statsRawBuckets {
		statsBuckets = append(statsBuckets, statsRawBucket.Export())
	}

	return &stats.Payload{
		HostName: tracePayload.HostName,
		Env:      tracePayload.Env,
		Stats:    statsBuckets,
	}
}
