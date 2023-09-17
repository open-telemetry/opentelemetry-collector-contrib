package pytransform

import (
	"context"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

type fakeMetricConsumer struct {
	pmetric.JSONMarshaler
	t        *testing.T
	expected string
}

func (f *fakeMetricConsumer) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	b, err := f.MarshalMetrics(md)
	require.NoError(f.t, err)
	require.Equal(f.t, f.expected, string(b))
	return nil
}

func (f *fakeMetricConsumer) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: true}
}

func TestMetricConsumer(t *testing.T) {
	t.Parallel()
	startLogServer(zap.NewNop())
	ep, err := getEmbeddedPython()
	require.NoError(t, err)
	next := &fakeMetricConsumer{t: t, expected: `{"resourceMetrics":[{"resource":{},"scopeMetrics":[{"scope":{"name":"otelcol/hostmetricsreceiver/memory","version":"0.84.0"},"metrics":[{"name":"system.memory.usage","description":"Bytes of memory in use.","unit":"By","sum":{"dataPoints":[{"attributes":[{"key":"state","value":{"stringValue":"used"}}],"startTimeUnixNano":"1694171569000000000","timeUnixNano":"1694189699786689531","asInt":"1874247680"},{"attributes":[{"key":"state","value":{"stringValue":"free"}}],"startTimeUnixNano":"1694171569000000000","timeUnixNano":"1694189699786689531","asInt":"29214199808"}],"aggregationTemporality":2}}]}],"schemaUrl":"https://opentelemetry.io/schemas/1.9.0"}]}`}
	mp := newMetricsProcessor(context.Background(), zap.NewNop(), createDefaultConfig().(*Config), ep, next)

	f, err := os.Open("testdata/metric_event_example.json")
	require.NoError(t, err)

	defer f.Close()

	b, err := io.ReadAll(f)
	require.NoError(t, err)

	md, err := (&pmetric.JSONUnmarshaler{}).UnmarshalMetrics(b)
	require.NoError(t, err)

	err = mp.ConsumeMetrics(context.Background(), md)
	require.NoError(t, err)
}
