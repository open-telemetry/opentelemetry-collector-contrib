// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and

package observability

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
func (e *exporter) ExportMetrics(ctx context.Context, data []*metricdata.Metric) error {
	for _, m := range data {
		e.pipe <- m
	}
	return nil
}

// Creates metrics reader and forward metrics from it to chData
// Sens empty structs to fail chan afterwards
func metricReader(chData chan []*metricdata.Metric, fail chan struct{}, count int) {
	reader := metricexport.NewReader()
	e := newExporter()
	ch := e.ReturnAfter(count)

	// Add a manual retry mechanism in case there's a hiccup reading the
	// metrics from producers in ReadAndExport(): we can wait for the metrics
	// to come instead of failing because they didn't come right away.
	for i := 0; i < 10; i++ {
		go reader.ReadAndExport(e)

		select {
		case <-time.After(500 * time.Millisecond):

		case data := <-ch:
			chData <- data
			return
		}
	}

	fail <- struct{}{}
}

func TestMetrics(t *testing.T) {
	const (
		receiver      = "sqlquery/my-name"
		queryReceiver = "query-0: select * from simple_logs"
	)
	type testCase struct {
		name       string
		recordFunc string
		value      int64
	}
	tests := []testCase{
		{
			name:       "receiver/accepted/log/records",
			recordFunc: "RecordAcceptedLogs",
			value:      10,
		},
	}

	var (
		fail   = make(chan struct{})
		chData = make(chan []*metricdata.Metric)
	)

	go metricReader(chData, fail, len(tests))

	for _, tt := range tests {
		switch tt.recordFunc {
		case "RecordAcceptedLogs":
			require.NoError(t, RecordAcceptedLogs(tt.value, receiver, queryReceiver))
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
			assert.Equal(t, d.Descriptor.Name, tt.name)
			require.Len(t, d.TimeSeries, 1)
			require.Len(t, d.TimeSeries[0].Points, 1)
			assert.Equal(t, d.TimeSeries[0].Points[0].Value, tt.value)
			require.Len(t, d.TimeSeries[0].LabelValues, 2)
			require.True(t, d.TimeSeries[0].LabelValues[0].Present)
			assert.Equal(t, d.TimeSeries[0].LabelValues[0].Value, queryReceiver)
			require.True(t, d.TimeSeries[0].LabelValues[1].Present)
			assert.Equal(t, d.TimeSeries[0].LabelValues[1].Value, receiver)
		})
	}
}
