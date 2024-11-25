// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusremotewriteexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusremotewriteexporter"

import (
	"errors"
	"math"
	"sort"
	"sync/atomic"

	"github.com/prometheus/prometheus/prompb"
)

type batchTimeSeriesState struct {
	// Track batch sizes sent to avoid over allocating huge buffers.
	// This helps in the case where large batches are sent to avoid allocating too much unused memory
	nextTimeSeriesBufferSize     atomic.Int64
	nextMetricMetadataBufferSize atomic.Int64
	nextRequestBufferSize        atomic.Int64
}

func newBatchTimeSericesState() *batchTimeSeriesState {
	state := &batchTimeSeriesState{
		nextTimeSeriesBufferSize:     atomic.Int64{},
		nextMetricMetadataBufferSize: atomic.Int64{},
		nextRequestBufferSize:        atomic.Int64{},
	}
	state.nextTimeSeriesBufferSize.Store(math.MaxInt64)
	state.nextMetricMetadataBufferSize.Store(math.MaxInt64)
	state.nextRequestBufferSize.Store(0)
	return state
}

// batchTimeSeries splits series into multiple batch write requests.
func batchTimeSeries(tsMap map[string]*prompb.TimeSeries, maxBatchByteSize int, m []*prompb.MetricMetadata, state *batchTimeSeriesState) ([]*prompb.WriteRequest, error) {
	if len(tsMap) == 0 {
		return nil, errors.New("invalid tsMap: cannot be empty map")
	}

	// Allocate a buffer size of at least 10, or twice the last # of requests we sent
	requests := make([]*prompb.WriteRequest, 0, max(10, state.nextRequestBufferSize.Load()))

	// Allocate a time series buffer 2x the last time series batch size or the length of the input if smaller
	tsArray := make([]prompb.TimeSeries, 0, min(state.nextTimeSeriesBufferSize.Load(), int64(len(tsMap))))
	sizeOfCurrentBatch := 0

	i := 0
	for _, v := range tsMap {
		sizeOfSeries := v.Size()

		if sizeOfCurrentBatch+sizeOfSeries >= maxBatchByteSize {
			state.nextTimeSeriesBufferSize.Store(int64(max(10, 2*len(tsArray))))
			wrapped := convertTimeseriesToRequest(tsArray)
			requests = append(requests, wrapped)

			tsArray = make([]prompb.TimeSeries, 0, min(state.nextTimeSeriesBufferSize.Load(), int64(len(tsMap)-i)))
			sizeOfCurrentBatch = 0
		}

		tsArray = append(tsArray, *v)
		sizeOfCurrentBatch += sizeOfSeries
		i++
	}

	if len(tsArray) != 0 {
		wrapped := convertTimeseriesToRequest(tsArray)
		requests = append(requests, wrapped)
	}

	// Allocate a metric metadata buffer 2x the last metric metadata batch size or the length of the input if smaller
	mArray := make([]prompb.MetricMetadata, 0, min(state.nextMetricMetadataBufferSize.Load(), int64(len(m))))
	sizeOfCurrentBatch = 0
	i = 0
	for _, v := range m {
		sizeOfM := v.Size()

		if sizeOfCurrentBatch+sizeOfM >= maxBatchByteSize {
			state.nextMetricMetadataBufferSize.Store(int64(max(10, 2*len(mArray))))
			wrapped := convertMetadataToRequest(mArray)
			requests = append(requests, wrapped)

			mArray = make([]prompb.MetricMetadata, 0, min(state.nextMetricMetadataBufferSize.Load(), int64(len(m)-i)))
			sizeOfCurrentBatch = 0
		}

		mArray = append(mArray, *v)
		sizeOfCurrentBatch += sizeOfM
		i++
	}

	if len(mArray) != 0 {
		wrapped := convertMetadataToRequest(mArray)
		requests = append(requests, wrapped)
	}

	state.nextRequestBufferSize.Store(int64(2 * len(requests)))
	return requests, nil
}

func convertTimeseriesToRequest(tsArray []prompb.TimeSeries) *prompb.WriteRequest {
	// the remote_write endpoint only requires the timeseries.
	// otlp defines it's own way to handle metric metadata
	return &prompb.WriteRequest{
		// Prometheus requires time series to be sorted by Timestamp to avoid out of order problems.
		// See:
		// * https://github.com/open-telemetry/wg-prometheus/issues/10
		// * https://github.com/open-telemetry/opentelemetry-collector/issues/2315
		Timeseries: orderBySampleTimestamp(tsArray),
	}
}

func convertMetadataToRequest(m []prompb.MetricMetadata) *prompb.WriteRequest {
	return &prompb.WriteRequest{
		Metadata: m,
	}
}

func orderBySampleTimestamp(tsArray []prompb.TimeSeries) []prompb.TimeSeries {
	for i := range tsArray {
		sL := tsArray[i].Samples
		sort.Slice(sL, func(i, j int) bool {
			return sL[i].Timestamp < sL[j].Timestamp
		})
	}
	return tsArray
}
