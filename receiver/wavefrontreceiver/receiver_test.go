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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
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
		want []pmetric.Metric
	}{
		{
			name: "single.line",
			msg:  "single.metric 1 1582231120 source=e2e\n",
			want: []pmetric.Metric{
				buildIntMetric(
					"single.metric",
					func() pcommon.Map {
						m := pcommon.NewMap()
						m.PutStr("source", "e2e")
						return m
					}(),
					1582231120,
					1,
				),
			},
		},
		{
			name: "single.line.no.newline",
			msg:  "single.metric 1 1582231120 source=e2e",
			want: []pmetric.Metric{
				buildIntMetric(
					"single.metric",
					func() pcommon.Map {
						m := pcommon.NewMap()
						m.PutStr("source", "e2e")
						return m
					}(),
					1582231120,
					1,
				),
			},
		},
		{
			name: "multiple.lines",
			msg:  "m0 0 1582231120 source=s0\nm1 1 1582231121 source=s1\nm2 2 1582231122 source=s2\n",
			want: []pmetric.Metric{
				buildIntMetric(
					"m0",
					func() pcommon.Map {
						m := pcommon.NewMap()
						m.PutStr("source", "s0")
						return m
					}(),
					1582231120,
					0,
				),
				buildIntMetric(
					"m1",
					func() pcommon.Map {
						m := pcommon.NewMap()
						m.PutStr("source", "s1")
						return m
					}(),
					1582231121,
					1,
				),
				buildIntMetric(
					"m2",
					func() pcommon.Map {
						m := pcommon.NewMap()
						m.PutStr("source", "s2")
						return m
					}(),
					1582231122,
					2,
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
		var gotMetrics []pmetric.Metric
		for _, md := range metrics {
			for i := 0; i < md.ResourceMetrics().Len(); i++ {
				rm := md.ResourceMetrics().At(i)
				for j := 0; j < rm.ScopeMetrics().Len(); j++ {
					sm := rm.ScopeMetrics().At(j)
					for k := 0; k < sm.Metrics().Len(); k++ {
						gotMetrics = append(gotMetrics, sm.Metrics().At(k))
					}
				}
			}
		}
		assert.Equal(t, tt.want, gotMetrics)
		sink.Reset()
	}
}
