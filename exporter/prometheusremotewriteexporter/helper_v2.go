// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusremotewriteexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusremotewriteexporter"

import (
	"errors"
	"sort"

	writev2 "github.com/prometheus/prometheus/prompb/io/prometheus/write/v2"
)

func batchTimeSeriesV2(tsMap map[string]*writev2.TimeSeries, symbolsTable writev2.SymbolsTable, maxBatchByteSize int, state *batchTimeSeriesState) ([]*writev2.Request, error) {
	if len(tsMap) == 0 {
		return nil, errors.New("invalid tsMap: cannot be empty map")
	}

	requests := make([]*writev2.Request, 0, max(10, state.nextRequestBufferSize))
	tsArray := make([]writev2.TimeSeries, 0, min(state.nextTimeSeriesBufferSize, len(tsMap)))

	// Calculate symbols table size once since it's shared across batches
	symbolsSize := 0
	for _, symbol := range symbolsTable.Symbols() {
		symbolsSize += len(symbol)
	}

	sizeOfCurrentBatch := symbolsSize // Initialize with symbols table size
	i := 0

	for _, v := range tsMap {
		sizeOfSeries := v.Size()

		if sizeOfCurrentBatch+sizeOfSeries >= maxBatchByteSize {
			state.nextTimeSeriesBufferSize = max(10, 2*len(tsArray))
			wrapped := convertTimeseriesToRequestV2(tsArray, symbolsTable)
			requests = append(requests, wrapped)

			tsArray = make([]writev2.TimeSeries, 0, min(state.nextTimeSeriesBufferSize, len(tsMap)-i))
			sizeOfCurrentBatch = symbolsSize // Reset to symbols table size for new batch
		}

		tsArray = append(tsArray, *v)
		sizeOfCurrentBatch += sizeOfSeries
		i++
	}

	if len(tsArray) != 0 {
		// TODO only sent necessary part of the symbolsTable
		wrapped := convertTimeseriesToRequestV2(tsArray, symbolsTable)
		requests = append(requests, wrapped)
	}

	state.nextRequestBufferSize = 2 * len(requests)
	return requests, nil
}

func convertTimeseriesToRequestV2(tsArray []writev2.TimeSeries, symbolsTable writev2.SymbolsTable) *writev2.Request {
	return &writev2.Request{
		// Prometheus requires time series to be sorted by Timestamp to avoid out of order problems.
		// See:
		// * https://github.com/open-telemetry/wg-prometheus/issues/10
		// * https://github.com/open-telemetry/opentelemetry-collector/issues/2315
		// TODO: try to sort while batching?
		Timeseries: orderBySampleTimestampV2(tsArray),
		Symbols:    symbolsTable.Symbols(),
	}
}

func orderBySampleTimestampV2(tsArray []writev2.TimeSeries) []writev2.TimeSeries {
	for i := range tsArray {
		sL := tsArray[i].Samples
		sort.Slice(sL, func(i, j int) bool {
			return sL[i].Timestamp < sL[j].Timestamp
		})
	}
	return tsArray
}
