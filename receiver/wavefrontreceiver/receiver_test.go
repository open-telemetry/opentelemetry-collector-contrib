// Copyright 2019, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package wavefrontreceiver

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/open-telemetry/opentelemetry-collector/component"
	"github.com/open-telemetry/opentelemetry-collector/consumer"
	"github.com/open-telemetry/opentelemetry-collector/consumer/consumerdata"
	"github.com/open-telemetry/opentelemetry-collector/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func Test_wavefrontreceiver_EndToEnd(t *testing.T) {
	factory := &Factory{}
	rCfg := factory.CreateDefaultConfig().(*Config)
	rCfg.ExtractCollectdTags = true
	rCfg.TCPIdleTimeout = time.Second

	addr := testutils.GetAvailableLocalAddress(t)
	rCfg.Endpoint = addr
	waitableConsumer := waitableMetricsConsumer{}
	rcvr, err := factory.CreateMetricsReceiver(zap.NewNop(), rCfg, &waitableConsumer)
	require.NoError(t, err)

	mh := component.NewMockHost()
	require.NoError(t, rcvr.Start(mh))
	defer rcvr.Shutdown()

	tests := []struct {
		name string
		msg  string
		want []*metricspb.Metric
	}{
		{
			name: "single.line",
			msg:  "single.metric 1 1582231120 source=e2e\n",
			want: []*metricspb.Metric{
				buildMetric(
					metricspb.MetricDescriptor_GAUGE_INT64,
					"single.metric",
					[]string{"source"},
					[]string{"e2e"},
					&metricspb.Point{
						Timestamp: &timestamp.Timestamp{Seconds: 1582231120},
						Value:     &metricspb.Point_Int64Value{Int64Value: 1},
					},
				),
			},
		},
		{
			name: "single.line.no.newline",
			msg:  "single.metric 1 1582231120 source=e2e",
			want: []*metricspb.Metric{
				buildMetric(
					metricspb.MetricDescriptor_GAUGE_INT64,
					"single.metric",
					[]string{"source"},
					[]string{"e2e"},
					&metricspb.Point{
						Timestamp: &timestamp.Timestamp{Seconds: 1582231120},
						Value:     &metricspb.Point_Int64Value{Int64Value: 1},
					},
				),
			},
		},
		{
			name: "multiple.lines",
			msg:  "m0 0 1582231120 source=s0\nm1 1 1582231121 source=s1\nm2 2 1582231122 source=s2\n",
			want: []*metricspb.Metric{
				buildMetric(
					metricspb.MetricDescriptor_GAUGE_INT64,
					"m0",
					[]string{"source"},
					[]string{"s0"},
					&metricspb.Point{
						Timestamp: &timestamp.Timestamp{Seconds: 1582231120},
						Value:     &metricspb.Point_Int64Value{Int64Value: 0},
					},
				),
				buildMetric(
					metricspb.MetricDescriptor_GAUGE_INT64,
					"m1",
					[]string{"source"},
					[]string{"s1"},
					&metricspb.Point{
						Timestamp: &timestamp.Timestamp{Seconds: 1582231121},
						Value:     &metricspb.Point_Int64Value{Int64Value: 1},
					},
				),
				buildMetric(
					metricspb.MetricDescriptor_GAUGE_INT64,
					"m2",
					[]string{"source"},
					[]string{"s2"},
					&metricspb.Point{
						Timestamp: &timestamp.Timestamp{Seconds: 1582231122},
						Value:     &metricspb.Point_Int64Value{Int64Value: 2},
					},
				),
			},
		},
	}
	for _, tt := range tests {
		conn, err := net.Dial("tcp", addr)
		require.NoError(t, err)

		numMetrics := strings.Count(tt.msg, "\n")
		if numMetrics == 0 {
			numMetrics = 1
		}
		waitableConsumer.Add(numMetrics)
		n, err := fmt.Fprint(conn, tt.msg)
		assert.Equal(t, len(tt.msg), n)
		assert.NoError(t, err)

		require.NoError(t, conn.Close())
		waitableConsumer.Wait()

		got := waitableConsumer.PullReceivedMetrics()
		assert.Equal(t, tt.want, got)
	}
}

type waitableMetricsConsumer struct {
	sync.WaitGroup
	mtx     sync.Mutex
	metrics []*metricspb.Metric
}

var _ (consumer.MetricsConsumer) = (*waitableMetricsConsumer)(nil)

func (w *waitableMetricsConsumer) ConsumeMetricsData(ctx context.Context, md consumerdata.MetricsData) error {
	w.mtx.Lock()
	defer w.mtx.Unlock()
	defer w.Done()
	w.metrics = append(w.metrics, md.Metrics...)
	return nil
}

func (w *waitableMetricsConsumer) PullReceivedMetrics() []*metricspb.Metric {
	w.mtx.Lock()
	defer w.mtx.Unlock()
	metrics := w.metrics
	w.metrics = nil
	return metrics
}
