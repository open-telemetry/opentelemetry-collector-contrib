// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusreceiver

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/require"

	"go.opentelemetry.io/collector/pdata/pmetric"

	dto "github.com/prometheus/prometheus/prompb/io/prometheus/client"
)

func TestScrapeViaProtobuf(t *testing.T) {
	// Create a Prometheus metric family and encode it to protobuf.
	mf := &dto.MetricFamily{
		Name: "test_counter",
		Type: dto.MetricType_COUNTER,
		Metric: []dto.Metric{
			{
				Label: []dto.LabelPair{
					{
						Name:  "foo",
						Value: "bar",
					},
				},
				Counter: &dto.Counter{
					Value: 1234,
				},
			},
		},
	}
	data, err := proto.Marshal(mf)
	require.NoError(t, err)

	buffer := &bytes.Buffer{}
	varintBuf := make([]byte, binary.MaxVarintLen32)
	varintLength := binary.PutUvarint(varintBuf, uint64(len(data)))

	_, err = buffer.Write(varintBuf[:varintLength])
	require.NoError(t, err)
	_, err = buffer.Write(data)
	require.NoError(t, err)

	targets := []*testData{
		{
			name: "target1",
			pages: []mockPrometheusResponse{
				{code: 200, useProtoBuf: true, buf: buffer.Bytes()},
			},
			validateFunc: func(t *testing.T, td *testData, result []pmetric.ResourceMetrics) {
				verifyNumValidScrapeResults(t, td, result)
				m1 := result[0]
				sm := m1.ScopeMetrics()
				require.Equal(t, 1, sm.Len())
				metrics := sm.At(0).Metrics()
				var m pmetric.Metric
				for i := 0; i < metrics.Len(); i++ {
					if metrics.At(i).Name() == "test_counter" {
						m = metrics.At(i)
					}
				}
				require.NotNil(t, m)
				require.Equal(t, pmetric.MetricTypeSum, m.Type())
				require.Equal(t, float64(1234), m.Sum().DataPoints().At(0).DoubleValue())
			},
		},
	}

	testComponent(t, targets, func(c *Config) {
		c.EnableProtobufNegotiation = true
	})
}