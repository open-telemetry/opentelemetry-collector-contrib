// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package serializeprofiles // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/serializer/otelserializer/serializeprofiles"

import (
	"fmt"
	"math/rand/v2"
)

// ## Why do we need downsampling ?
// For every (virtual) CPU core, the host agent (HA) retrieves 20 stacktrace events which are
// stored as a timeseries in an ES index (name 'profiling-events'). With an increasing number
// of hosts and/or an increasing number of cores the number of stored events per second
// become high quickly. E.g. data from 10000 cores generate 846 million events per day.
// Since users want to drill down into e.g. single hosts and/or single applications, we can't
// reduce the amount of data in advance. Querying such amounts data is costly, even when using
// highly specialised database backends - costly in terms of I/O, CPU. And this results in
// increased latency - the user has to wait eventually a long time for his query results.
// In order to reduce the costs and to keep the latency as low as possible, we add 'helper'
// indexes with downsampled subsets of the stacktrace events.
//
// ## How does our downsampling approach work ?
// The idea is to create downsampled indexes with factors of 5^N (5, 25, 125, 625, ...).
// In the 5^1 index we would store 1/5th of the original events, in the 5^2 index we store
// 1/25th of the original events and so on.
// So each event has a probability of p=1/5=0.2 to also be stored in the next downsampled index.
// Since we aggregate identical stacktrace events by timestamp when reported and stored, we have
// a 'Count' value for each. To be statistically correct, we have to apply p=0.2 to each single
// event independently and not just to the aggregate. We can do so by looping over 'Count' and
// apply p=0.2 on every iteration to generate a new 'Count' value for the next downsampled index.
// We only store aggregates with 'Count' > 0.
//
// At some point we decided that 20k events per query is good enough. With 5^N it means that we
// possibly can end up with 5x more events (100k) from an index. As of this writing, retrieving
// and processing of 100k events is still fast enough. While in Clickhouse we could further
// downsample on-the-fly to get 20k, ES currently doesn't allow this (may change in the future).
//
// At query time we have to find the index that has enough data to be statistically sound,
// without having too much data to avoid costs and latency. The code for that is implemented on
// the read path (Kibana profiler plugin) and described there in detail.
//
// ## Example of a query / calculation
// Let's imagine, a given query spans a time range of 7 days and would result in 100 million
// events without down-sampling. But we only really need 20k events for a good enough result.
// In the 5^1 downsampled index we have 5x less data - this still results in 20 millions events.
// Going deeper we end up in the 5^5 downsampled index with 32k results - 5^4 would give us 160k
// (too many) and 5^6 would give us 6.4k events (not enough).
// We now read and process all 32k events from the 5^5 index. The counts for any aggregation
// (TopN, Flamegraph, ...) needs to be multiplied by 5^5, which is an estimate of what we would
// have found in the full events index (the not downsampled index).
//
// ## How deep do we have to downsample ?
// The current code assumes an arbitrary upper limit of 100k CPU cores and a query time range
// of 7 days. (Please be aware that we get 20 events per core per second only if the core is
// 100% busy.)
//
// The condition is
//
//	(100k * 86400 * 7 * 20) / 5^N in [20k, 100k-1]
//	                    ^-- max number of events per second
//	                ^------ number of days
//	       	^-------------- seconds per day
//	 ^--------------------- number of cores
//
// For N=11 the condition is satisfied with a value of 24772.
// In numbers, the 5^11 downsampled index holds 48828125x fewer entries than the full events table.
//
// ## What is the cost of downsampling ?
// The additional cost in terms of storage size is
//
//	1/5^1 +1/5^2 + ... + 1/5^11 = 25%
//
// The same goes for the additional CPU cost on the write path.
//
// The average benefit on the read/query path depends on the query. But it seems that in average
// a factor of few hundred to a few thousand in terms of I/O, CPU and latency can be achieved.
const (
	maxEventsIndexes = 11
	samplingFactor   = 5
	samplingRatio    = 1.0 / float64(samplingFactor)

	eventsIndexPrefix = "profiling-events"
)

var eventIndices = initEventIndexes(maxEventsIndexes)

// A fixed seed is used for deterministic tests and development.
// There is no downside in using a fixed seed in production.
var rnd = rand.New(rand.NewPCG(0, 0))

// initEventIndexes initializes eventIndexes to avoid calculations for every TraceEvent later.
func initEventIndexes(count int) []string {
	indices := make([]string, 0, count)

	for i := range count {
		indices = append(indices, fmt.Sprintf("%s-%dpow%02d",
			eventsIndexPrefix, samplingFactor, i+1))
	}

	return indices
}

func IndexDownsampledEvent(event StackTraceEvent, pushData func(any, string, string) error) error {
	// Each event has a probability of p=1/5=0.2 to go from one index into the next downsampled
	// index. Since we aggregate identical stacktrace events by timestamp when reported and stored,
	// we have a 'Count' value for each. To be statistically correct, we have to apply p=0.2 to
	// each single stacktrace event independently and not just to the aggregate. We can do so by
	// looping over 'Count' and apply p=0.2 on every iteration to generate a new 'Count' value for
	// the next downsampled index.
	// We only store aggregates with 'Count' > 0. If 'Count' becomes 0, we are done and can
	// continue with the next stacktrace event.
	for _, index := range eventIndices {
		var count uint16
		for range event.Count {
			// samplingRatio is the probability p=0.2 for an event to be copied into the next
			// downsampled index.
			if rnd.Float64() < samplingRatio {
				count++
			}
		}
		if count == 0 {
			return nil
		}

		// Store the event with its new downsampled count in the downsampled index.
		event.Count = count

		if err := pushData(event, "", index); err != nil {
			return err
		}
	}

	return nil
}
