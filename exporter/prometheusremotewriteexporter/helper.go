// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusremotewriteexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusremotewriteexporter"

import (
	"errors"
	"sort"

	"github.com/prometheus/prometheus/prompb"
)

// batchTimeSeries splits series into multiple batch write requests.
func batchTimeSeries(timeSeries []prompb.TimeSeries, maxBatchByteSize int, m []*prompb.MetricMetadata) ([]*prompb.WriteRequest, error) {
	if len(timeSeries) == 0 {
		return nil, errors.New("invalid timeSeries: cannot be empty")
	}

	requests := make([]*prompb.WriteRequest, 0, len(timeSeries)+len(m))
	tsArray := make([]prompb.TimeSeries, 0, len(timeSeries))
	sizeOfCurrentBatch := 0

	i := 0
	for _, ts := range timeSeries {
		sizeOfSeries := ts.Size()

		if sizeOfCurrentBatch+sizeOfSeries >= maxBatchByteSize {
			wrapped := convertTimeseriesToRequest(tsArray)
			requests = append(requests, wrapped)

			tsArray = make([]prompb.TimeSeries, 0, len(timeSeries)-i)
			sizeOfCurrentBatch = 0
		}

		tsArray = append(tsArray, ts)
		sizeOfCurrentBatch += sizeOfSeries
		i++
	}

	if len(tsArray) != 0 {
		wrapped := convertTimeseriesToRequest(tsArray)
		requests = append(requests, wrapped)
	}

	mArray := make([]prompb.MetricMetadata, 0, len(m))
	sizeOfCurrentBatch = 0
	i = 0
	for _, v := range m {
		sizeOfM := v.Size()

		if sizeOfCurrentBatch+sizeOfM >= maxBatchByteSize {
			wrapped := convertMetadataToRequest(mArray)
			requests = append(requests, wrapped)

			mArray = make([]prompb.MetricMetadata, 0, len(m)-i)
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
