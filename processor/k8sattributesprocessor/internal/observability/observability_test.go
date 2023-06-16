// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package observability

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opencensus.io/metric/metricdata"
	"go.opencensus.io/metric/metricexport"
)

type exporter struct {
	pipe chan *metricdata.Metric
}

func newExporter() *exporter {
	return &exporter{make(chan *metricdata.Metric)}
}

func (e *exporter) ReturnAfter(after int) chan []*metricdata.Metric {
	ch := make(chan []*metricdata.Metric)
	go func() {
		// Sometimes we can get only subset of all metrics, so lets make sure
		// we look at unique records
		receivedUnique := map[string]*metricdata.Metric{}
		for m := range e.pipe {
			receivedUnique[m.Descriptor.Name] = m
			if len(receivedUnique) >= after {
				break
			}
		}

		var received []*metricdata.Metric
		for _, value := range receivedUnique {
			received = append(received, value)
		}
		ch <- received
	}()
	return ch
}

func (e *exporter) ExportMetrics(_ context.Context, data []*metricdata.Metric) error {
	for _, m := range data {
		e.pipe <- m
	}
	return nil
}

// NOTE:
// This test can only be run with -count 1 because of static
// metricproducer.GlobalManager() used in metricexport.NewReader().
func TestMetrics(t *testing.T) {
	type testCase struct {
		name       string
		recordFunc func()
	}
	tests := []testCase{
		{
			"processor.k8sattributes.pods.added",
			RecordPodAdded,
		},
		{
			"processor.k8sattributes.pods.deleted",
			RecordPodDeleted,
		},
		{
			"processor.k8sattributes.pods.updated",
			RecordPodUpdated,
		},
		{
			"processor.k8sattributes.ip_lookup_misses",
			RecordIPLookupMiss,
		},
		{
			"processor.k8sattributes.namespaces.added",
			RecordNamespaceAdded,
		},
		{
			"processor.k8sattributes.namespaces.updated",
			RecordNamespaceUpdated,
		},
		{
			"processor.k8sattributes.namespaces.deleted",
			RecordNamespaceDeleted,
		},
	}

	var (
		fail   = make(chan struct{})
		chData = make(chan []*metricdata.Metric)
	)

	go func() {
		reader := metricexport.NewReader()
		e := newExporter()
		ch := e.ReturnAfter(len(tests))

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
	}()

	for _, tt := range tests {
		tt.recordFunc()
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

	assert.Len(t, data, len(tests))

	for i, tt := range tests {
		d := data[i]
		assert.Equal(t, tt.name, d.Descriptor.Name)
		assert.Len(t, d.TimeSeries, 1)
		assert.Len(t, d.TimeSeries[0].Points, 1)
		assert.Equal(t, int64(1), d.TimeSeries[0].Points[0].Value)
	}
}
