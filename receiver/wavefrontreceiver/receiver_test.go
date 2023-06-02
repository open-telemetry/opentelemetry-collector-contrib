// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package wavefrontreceiver

import (
	"context"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
	internaldata "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/opencensus"
)

func Test_wavefrontreceiver_EndToEnd(t *testing.T) {
	rCfg := createDefaultConfig().(*Config)
	rCfg.ExtractCollectdTags = true
	rCfg.TCPIdleTimeout = time.Second

	addr := testutil.GetAvailableLocalAddress(t)
	rCfg.Endpoint = addr
	sink := new(consumertest.MetricsSink)
	params := receivertest.NewNopCreateSettings()
	rcvr, err := createMetricsReceiver(context.Background(), params, rCfg, sink)
	require.NoError(t, err)

	require.NoError(t, rcvr.Start(context.Background(), componenttest.NewNopHost()))

	defer func() {
		assert.NoError(t, rcvr.Shutdown(context.Background()))
	}()

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
						Timestamp: &timestamppb.Timestamp{Seconds: 1582231120},
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
						Timestamp: &timestamppb.Timestamp{Seconds: 1582231120},
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
						Timestamp: &timestamppb.Timestamp{Seconds: 1582231120},
						Value:     &metricspb.Point_Int64Value{Int64Value: 0},
					},
				),
				buildMetric(
					metricspb.MetricDescriptor_GAUGE_INT64,
					"m1",
					[]string{"source"},
					[]string{"s1"},
					&metricspb.Point{
						Timestamp: &timestamppb.Timestamp{Seconds: 1582231121},
						Value:     &metricspb.Point_Int64Value{Int64Value: 1},
					},
				),
				buildMetric(
					metricspb.MetricDescriptor_GAUGE_INT64,
					"m2",
					[]string{"source"},
					[]string{"s2"},
					&metricspb.Point{
						Timestamp: &timestamppb.Timestamp{Seconds: 1582231122},
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
		n, err := fmt.Fprint(conn, tt.msg)
		assert.Equal(t, len(tt.msg), n)
		assert.NoError(t, err)

		require.NoError(t, conn.Close())
		assert.Eventually(t, func() bool {
			return sink.DataPointCount() == numMetrics
		}, 10*time.Second, 5*time.Millisecond)

		metrics := sink.AllMetrics()
		var gotOldMetrics []*metricspb.Metric
		for _, md := range metrics {
			for i := 0; i < md.ResourceMetrics().Len(); i++ {
				_, _, metrics := internaldata.ResourceMetricsToOC(md.ResourceMetrics().At(i))
				gotOldMetrics = append(gotOldMetrics, metrics...)
			}
		}
		assert.Equal(t, tt.want, gotOldMetrics)
		sink.Reset()
	}
}
