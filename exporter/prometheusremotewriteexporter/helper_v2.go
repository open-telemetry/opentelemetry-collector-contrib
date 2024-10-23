// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusremotewriteexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusremotewriteexporter"

import (
	"errors"
	writev2 "github.com/prometheus/prometheus/prompb/io/prometheus/write/v2"
)

// batchTimeSeries splits series into multiple batch write requests.
func batchTimeSeriesV2(tsMap map[string]*writev2.TimeSeries, symbolsTable writev2.SymbolsTable, maxBatchByteSize int, state *batchTimeSeriesState) ([]*writev2.Request, error) {
	if len(tsMap) == 0 {
		return nil, errors.New("invalid tsMap: cannot be empty map")
	}

	// Allocate a buffer size of at least 10, or twice the last # of requests we sent
	requests := make([]*writev2.Request, 0, max(10, state.nextRequestBufferSize))

	// Allocate a time series buffer 2x the last time series batch size or the length of the input if smaller
	tsArray := make([]writev2.TimeSeries, 0, min(state.nextTimeSeriesBufferSize, len(tsMap)))
	sizeOfCurrentBatch := 0

	i := 0
	for _, v := range tsMap {
		sizeOfSeries := v.Size()

		if sizeOfCurrentBatch+sizeOfSeries >= maxBatchByteSize {
			state.nextTimeSeriesBufferSize = max(10, 2*len(tsArray))
			wrapped := convertTimeseriesToRequestV2(tsArray, symbolsTable)
			requests = append(requests, wrapped)

			tsArray = make([]writev2.TimeSeries, 0, min(state.nextTimeSeriesBufferSize, len(tsMap)-i))
			sizeOfCurrentBatch = 0
		}

		tsArray = append(tsArray, *v)
		sizeOfCurrentBatch += sizeOfSeries
		i++
	}

	if len(tsArray) != 0 {
		wrapped := convertTimeseriesToRequestV2(tsArray, symbolsTable)
		requests = append(requests, wrapped)
	}

	state.nextRequestBufferSize = 2 * len(requests)
	return requests, nil
}

func convertTimeseriesToRequestV2(tsArray []writev2.TimeSeries, symbolsTable writev2.SymbolsTable) *writev2.Request {
	// the remote_write endpoint only requires the timeseries.
	// otlp defines it's own way to handle metric metadata
	return &writev2.Request{
		// TODO sort
		// Prometheus requires time series to be sorted by Timestamp to avoid out of order problems.
		// See:
		// * https://github.com/open-telemetry/wg-prometheus/issues/10
		// * https://github.com/open-telemetry/opentelemetry-collector/issues/2315
		//Timeseries: orderBySampleTimestamp(tsArray),
		Timeseries: tsArray,
		Symbols:    symbolsTable.Symbols(),
	}
}
