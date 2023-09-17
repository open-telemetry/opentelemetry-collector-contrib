package pytransform

import (
	"context"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

type fakeTraceConsumer struct {
	ptrace.JSONMarshaler
	t        *testing.T
	expected string
}

func (f *fakeTraceConsumer) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	b, err := f.MarshalTraces(td)
	require.NoError(f.t, err)
	require.Equal(f.t, f.expected, string(b))
	return nil
}

func (f *fakeTraceConsumer) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func TestTraceConsumer(t *testing.T) {
	t.Parallel()
	startLogServer(zap.NewNop())
	ep, err := getEmbeddedPython()
	require.NoError(t, err)
	next := &fakeTraceConsumer{t: t, expected: `{"resourceSpans":[{"resource":{"attributes":[{"key":"telemetry.sdk.language","value":{"stringValue":"python"}},{"key":"telemetry.sdk.name","value":{"stringValue":"opentelemetry"}},{"key":"telemetry.sdk.version","value":{"stringValue":"1.19.0"}},{"key":"telemetry.auto.version","value":{"stringValue":"0.40b0"}},{"key":"service.name","value":{"stringValue":"unknown_service"}}]},"scopeSpans":[{"scope":{"name":"opentelemetry.instrumentation.flask","version":"0.40b0"},"spans":[{"traceId":"9cb5bf738137b2248dc7b20445ec2e1c","spanId":"88079ad5c94b5b13","parentSpanId":"","name":"/roll","kind":2,"startTimeUnixNano":"1694388218052842000","endTimeUnixNano":"1694388218053415000","attributes":[{"key":"http.method","value":{"stringValue":"GET"}},{"key":"http.server_name","value":{"stringValue":"0.0.0.0"}},{"key":"http.scheme","value":{"stringValue":"http"}},{"key":"net.host.port","value":{"intValue":"5001"}},{"key":"http.host","value":{"stringValue":"localhost:5001"}},{"key":"http.target","value":{"stringValue":"/roll"}},{"key":"net.peer.ip","value":{"stringValue":"127.0.0.1"}},{"key":"http.user_agent","value":{"stringValue":"curl/7.87.0"}},{"key":"net.peer.port","value":{"intValue":"52365"}},{"key":"http.flavor","value":{"stringValue":"1.1"}},{"key":"http.route","value":{"stringValue":"/roll"}},{"key":"http.status_code","value":{"intValue":"200"}}],"status":{}}]}]}]}`}
	tp := newTraceProcessor(context.Background(), zap.NewNop(), createDefaultConfig().(*Config), ep, next)

	f, err := os.Open("testdata/trace_event_example.json")
	require.NoError(t, err)

	b, err := io.ReadAll(f)
	require.NoError(t, err)

	td, err := (&ptrace.JSONUnmarshaler{}).UnmarshalTraces(b)
	require.NoError(t, err)

	err = tp.ConsumeTraces(context.Background(), td)
	require.NoError(t, err)
}
