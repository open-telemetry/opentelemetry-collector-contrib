// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metricgroup

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/datapoints"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/elasticsearch"
)

func TestECSDataPointHasher_HashKey(t *testing.T) {
	ts := pcommon.NewTimestampFromTime(time.Unix(1_700_000_000, 0))

	cases := []struct {
		name string
		// left and right are two (resource, datapoint) inputs whose HashKeys
		// are compared. wantSame is true when they should group together (same
		// identity).
		left     func() (pcommon.Resource, datapoints.DataPoint)
		right    func() (pcommon.Resource, datapoints.DataPoint)
		wantSame bool
	}{
		{
			name: "dp attr overwrites resource attr",
			left: func() (pcommon.Resource, datapoints.DataPoint) {
				r := pcommon.NewResource()
				r.Attributes().PutStr("service.name", "from-resource")
				r.Attributes().PutStr("host.name", "host-a")
				return r, newNumberDP(ts, map[string]string{"service.name": "from-dp"})
			},
			right: func() (pcommon.Resource, datapoints.DataPoint) {
				// Equivalent merged attrs: service.name=from-dp, host.name=host-a
				r := pcommon.NewResource()
				r.Attributes().PutStr("host.name", "host-a")
				return r, newNumberDP(ts, map[string]string{"service.name": "from-dp"})
			},
			wantSame: true,
		},
		{
			name: "attr insertion order does not matter",
			left: func() (pcommon.Resource, datapoints.DataPoint) {
				r := pcommon.NewResource()
				r.Attributes().PutStr("a", "1")
				r.Attributes().PutStr("b", "2")
				return r, newNumberDP(ts, map[string]string{"c": "3"})
			},
			right: func() (pcommon.Resource, datapoints.DataPoint) {
				r := pcommon.NewResource()
				r.Attributes().PutStr("b", "2")
				r.Attributes().PutStr("a", "1")
				return r, newNumberDP(ts, map[string]string{"c": "3"})
			},
			wantSame: true,
		},
		{
			name: "different timestamp yields different hash",
			left: func() (pcommon.Resource, datapoints.DataPoint) {
				r := pcommon.NewResource()
				r.Attributes().PutStr("host.name", "host-a")
				return r, newNumberDP(ts, nil)
			},
			right: func() (pcommon.Resource, datapoints.DataPoint) {
				r := pcommon.NewResource()
				r.Attributes().PutStr("host.name", "host-a")
				otherTS := pcommon.NewTimestampFromTime(time.Unix(1_700_000_001, 0))
				return r, newNumberDP(otherTS, nil)
			},
			wantSame: false,
		},
		{
			name: "reserved data_stream attrs excluded",
			left: func() (pcommon.Resource, datapoints.DataPoint) {
				r := pcommon.NewResource()
				r.Attributes().PutStr("host.name", "host-a")
				r.Attributes().PutStr(elasticsearch.DataStreamType, "metrics")
				r.Attributes().PutStr(elasticsearch.DataStreamDataset, "generic")
				r.Attributes().PutStr(elasticsearch.DataStreamNamespace, "default")
				return r, newNumberDP(ts, nil)
			},
			right: func() (pcommon.Resource, datapoints.DataPoint) {
				r := pcommon.NewResource()
				r.Attributes().PutStr("host.name", "host-a")
				return r, newNumberDP(ts, nil)
			},
			wantSame: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			leftRes, leftDP := tc.left()
			rightRes, rightDP := tc.right()

			leftHasher := &ECSDataPointHasher{}
			leftHasher.UpdateResource(leftRes)
			leftHasher.UpdateDataPoint(leftDP)

			rightHasher := &ECSDataPointHasher{}
			rightHasher.UpdateResource(rightRes)
			rightHasher.UpdateDataPoint(rightDP)

			if tc.wantSame {
				require.Equal(t, leftHasher.HashKey(), rightHasher.HashKey())
			} else {
				require.NotEqual(t, leftHasher.HashKey(), rightHasher.HashKey())
			}
		})
	}
}

func newNumberDP(ts pcommon.Timestamp, attrs map[string]string) datapoints.DataPoint {
	metric := pmetric.NewMetric()
	metric.SetName("m")
	dp := metric.SetEmptyGauge().DataPoints().AppendEmpty()
	dp.SetTimestamp(ts)
	dp.SetDoubleValue(1)
	for k, v := range attrs {
		dp.Attributes().PutStr(k, v)
	}
	return datapoints.NewNumber(metric, dp)
}
