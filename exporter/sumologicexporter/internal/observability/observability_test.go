// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package observability // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/sumologicexporter/internal/observability"

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opencensus.io/metric/metricdata"
	"go.opencensus.io/metric/metricexport"
)

type exporter struct {
	pipe chan *metricdata.Metric
}

func newExporter() *exporter {
	return &exporter{
		make(chan *metricdata.Metric),
	}
}

// Run goroutine which is going to receive `after` metrics.
// Goroutine is writing to returned chan
func (e *exporter) ReturnAfter(after int) chan []*metricdata.Metric {
	ch := make(chan []*metricdata.Metric)
	go func() {
		received := []*metricdata.Metric{}
		for m := range e.pipe {
			received = append(received, m)
			if len(received) >= after {
				break
			}
		}
		ch <- received
	}()
	return ch
}

// Write received metrics to data channel
func (e *exporter) ExportMetrics(_ context.Context, data []*metricdata.Metric) error {
	for _, m := range data {
		e.pipe <- m
	}
	return nil
}

// Creates metrics reader and forward metrics from it to chData
// Sens empty structs to fail chan afterwards
func metricReader(chData chan []*metricdata.Metric, fail chan struct{}, count int) {

	// Add a manual retry mechanism in case there's a hiccup reading the
	// metrics from producers in ReadAndExport(): we can wait for the metrics
	// to come instead of failing because they didn't come right away.
	for i := 0; i < 10; i++ {
		e := newExporter()
		ch := e.ReturnAfter(count)
		go metricexport.NewReader().ReadAndExport(e)

		select {
		case <-time.After(500 * time.Millisecond):

		case data := <-ch:
			chData <- data
			return
		}
	}

	fail <- struct{}{}
}

// NOTE:
// This test can only be run with -count 1 because of static
// metricproducer.GlobalManager() used in metricexport.NewReader().
func TestMetrics(t *testing.T) {
	const (
		statusCode   = 200
		endpoint     = "some/uri"
		pipeline     = "metrics"
		exporter     = "sumologic/my-name"
		bytesFunc    = "bytes"
		recordsFunc  = "records"
		durationFunc = "duration"
		sentFunc     = "sent"
	)
	type testCase struct {
		name       string
		bytes      int64
		records    int64
		recordFunc string
		duration   time.Duration
	}
	tests := []testCase{
		{
			name:       "exporter/requests/sent",
			recordFunc: sentFunc,
		},
		{
			name:       "exporter/requests/duration",
			recordFunc: durationFunc,
			duration:   time.Millisecond,
		},
		{
			name:       "exporter/requests/bytes",
			recordFunc: bytesFunc,
			bytes:      1,
		},
		{
			name:       "exporter/requests/records",
			recordFunc: recordsFunc,
			records:    1,
		},
	}

	var (
		fail   = make(chan struct{})
		chData = make(chan []*metricdata.Metric)
	)

	go metricReader(chData, fail, len(tests))

	for _, tt := range tests {
		switch tt.recordFunc {
		case sentFunc:
			require.NoError(t, RecordRequestsSent(statusCode, endpoint, pipeline, exporter))
		case durationFunc:
			require.NoError(t, RecordRequestsDuration(tt.duration, statusCode, endpoint, pipeline, exporter))
		case bytesFunc:
			require.NoError(t, RecordRequestsBytes(tt.bytes, statusCode, endpoint, pipeline, exporter))
		case recordsFunc:
			require.NoError(t, RecordRequestsRecords(tt.records, statusCode, endpoint, pipeline, exporter))
		}
	}

	var data []*metricdata.Metric
	select {
	case <-fail:
		t.Fatalf("timedout waiting for metrics to arrive")
	case data = <-chData:
	}

	sort.Slice(tests, func(i, j int) bool {
		return tests[i].name < tests[j].name
	})

	sort.Slice(data, func(i, j int) bool {
		return data[i].Descriptor.Name < data[j].Descriptor.Name
	})

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Len(t, data, len(tests))
			d := data[i]
			assert.Equal(t, tt.name, d.Descriptor.Name, "Expected %v at index %v, but got %v.", tt.name, i, d.Descriptor.Name)
			require.Len(t, d.TimeSeries, 1)
			require.Len(t, d.TimeSeries[0].Points, 1)
			assert.Equal(t, d.TimeSeries[0].Points[0].Value, int64(1))

			require.Len(t, d.TimeSeries[0].LabelValues, 4)

			require.True(t, d.TimeSeries[0].LabelValues[0].Present)
			require.True(t, d.TimeSeries[0].LabelValues[1].Present)
			require.True(t, d.TimeSeries[0].LabelValues[2].Present)
			require.True(t, d.TimeSeries[0].LabelValues[3].Present)

			assert.Equal(t, d.TimeSeries[0].LabelValues[0].Value, "some/uri")
			assert.Equal(t, d.TimeSeries[0].LabelValues[1].Value, "sumologic/my-name")
			assert.Equal(t, d.TimeSeries[0].LabelValues[2].Value, "metrics")
			assert.Equal(t, d.TimeSeries[0].LabelValues[3].Value, "200")
		})
	}
}
