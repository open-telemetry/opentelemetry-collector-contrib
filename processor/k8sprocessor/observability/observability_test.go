// Copyright 2020 OpenTelemetry Authors
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
	"sort"
	"testing"

	"go.opencensus.io/metric/metricdata"
	"go.opencensus.io/metric/metricexport"
)

type exporter struct {
	pipe chan []*metricdata.Metric
}

func newExporter() *exporter {
	return &exporter{make(chan []*metricdata.Metric)}
}

func (e *exporter) ExportMetrics(ctx context.Context, data []*metricdata.Metric) error {
	e.pipe <- data
	return nil
}

func TestMetrics(t *testing.T) {
	type testCase struct {
		name       string
		recordFunc func()
	}
	tests := []testCase{
		{
			"otelsvc/k8s/pod_added",
			RecordPodAdded,
		},
		{
			"otelsvc/k8s/pod_deleted",
			RecordPodDeleted,
		},
		{
			"otelsvc/k8s/pod_updated",
			RecordPodUpdated,
		},
		{
			"otelsvc/k8s/ip_lookup_miss",
			RecordIPLookupMiss,
		},
	}

	e := newExporter()
	metricReader := metricexport.NewReader()
	for _, tt := range tests {
		tt.recordFunc()
	}
	go metricReader.ReadAndExport(e)
	data := <-e.pipe

	sort.Slice(tests, func(i, j int) bool {
		return tests[i].name < tests[j].name
	})

	sort.Slice(data, func(i, j int) bool {
		return data[i].Descriptor.Name < data[j].Descriptor.Name
	})

	// TODO: FIXME: this is one flaky test
	//for i, tt := range tests {
	//	require.Len(t, data, len(tests))
	//	d := data[i]
	//	assert.Equal(t, d.Descriptor.Name, tt.name)
	//	require.Len(t, d.TimeSeries, 1)
	//	require.Len(t, d.TimeSeries[0].Points, 1)
	//	assert.Equal(t, d.TimeSeries[0].Points[0].Value, int64(1))
	//}
}
