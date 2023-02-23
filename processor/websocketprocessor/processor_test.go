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
// limitations under the License.

package websocketprocessor

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/processor/processortest"
	"golang.org/x/net/nettest"
	"golang.org/x/net/websocket"
)

func TestProcessor(t *testing.T) {
	listener, err := nettest.NewLocalListener("tcp4")
	require.NoError(t, err)
	addr := listener.Addr()
	ctx := context.Background()
	tc := &testConsumer{}
	proc, err := newMetricsProcessor(processortest.NewNopCreateSettings(), tc, listener, 0)
	require.NoError(t, err)
	err = proc.Start(ctx, componenttest.NewNopHost())
	require.NoError(t, err)
	ws, err := websocket.Dial(fmt.Sprintf("ws://%s", addr), "", "http://localhost")
	require.NoError(t, err)
	metrics := testMetrics()
	err = proc.ConsumeMetrics(context.Background(), metrics)
	var buf = make([]byte, 512)
	_, err = ws.Read(buf)
	require.NoError(t, err)
	got, err := (&pmetric.JSONUnmarshaler{}).UnmarshalMetrics(buf)
	require.NoError(t, err)
	err = pmetrictest.CompareMetrics(metrics, got)
	require.NoError(t, err)
}

func testMetrics() pmetric.Metrics {
	out := pmetric.NewMetrics()
	out.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics().AppendEmpty().SetEmptyGauge().DataPoints().AppendEmpty().SetIntValue(42)
	return out
}

type testConsumer struct {
	mu sync.Mutex
	a  []pmetric.Metrics
}

func (t *testConsumer) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{}
}

func (t *testConsumer) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	t.mu.Lock()
	t.a = append(t.a, md)
	t.mu.Unlock()
	return nil
}

func (t *testConsumer) getMetrics() []pmetric.Metrics {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.a
}
