package pytransform

import (
	"context"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"
)

type fakeLogConsumer struct {
	plog.JSONMarshaler
	t        *testing.T
	expected string
}

func (f *fakeLogConsumer) ConsumeLogs(ctx context.Context, ld plog.Logs) error {
	b, err := f.MarshalLogs(ld)
	require.NoError(f.t, err)
	require.Equal(f.t, f.expected, string(b))
	return nil
}

func (f *fakeLogConsumer) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func TestLogConsumer(t *testing.T) {
	t.Parallel()
	startLogServer(zap.NewNop())
	ep, err := getEmbeddedPython()
	require.NoError(t, err)
	next := &fakeLogConsumer{t: t, expected: `{"resourceLogs":[{"resource":{"attributes":[{"key":"log.file.name","value":{"stringValue":"test.log"}}]},"scopeLogs":[{"scope":{},"logRecords":[{"observedTimeUnixNano":"1694127596456358000","body":{"stringValue":"2023-09-06T01:09:24.045+0200    INFO    internal/command.go:117 OpenTelemetry Collector Builder {\"version\": \"0.84.0\", \"date\": \"2023-08-29T18:58:24Z\"}"},"attributes":[{"key":"app","value":{"stringValue":"dev"}}],"traceId":"","spanId":""},{"observedTimeUnixNano":"1694127596456543000","body":{"stringValue":"2023-09-06T01:09:24.049+0200    INFO    internal/command.go:150 Using config file       {\"path\": \"builder.yaml\"}"},"attributes":[{"key":"app","value":{"stringValue":"dev"}}],"traceId":"","spanId":""},{"observedTimeUnixNano":"1694127596456572000","body":{"stringValue":"2023-09-06T01:09:24.049+0200    INFO    builder/config.go:106   Using go        {\"go-executable\": \"/usr/local/go/bin/go\"}"},"attributes":[{"key":"app","value":{"stringValue":"dev"}}],"traceId":"","spanId":""},{"observedTimeUnixNano":"1694127596456576000","body":{"stringValue":"2023-09-06T01:09:24.050+0200    INFO    builder/main.go:69      Sources created {\"path\": \"./cmd/ddot\"}"},"attributes":[{"key":"app","value":{"stringValue":"dev"}}],"traceId":"","spanId":""},{"observedTimeUnixNano":"1694127596456611000","body":{"stringValue":"2023-09-06T01:09:35.392+0200    INFO    builder/main.go:121     Getting go modules"},"attributes":[{"key":"app","value":{"stringValue":"dev"}}],"traceId":"","spanId":""},{"observedTimeUnixNano":"1694127596456668000","body":{"stringValue":"2023-09-06T01:09:38.554+0200    INFO    builder/main.go:80      Compiling"},"attributes":[{"key":"app","value":{"stringValue":"dev"}}],"traceId":"","spanId":""}]}]}]}`}
	lp := newLogsProcessor(context.Background(), zap.NewNop(), createDefaultConfig().(*Config), ep, next)

	f, err := os.Open("testdata/log.json")
	require.NoError(t, err)

	b, err := io.ReadAll(f)
	require.NoError(t, err)

	ld, err := (&plog.JSONUnmarshaler{}).UnmarshalLogs(b)
	require.NoError(t, err)

	err = lp.ConsumeLogs(context.Background(), ld)
	require.NoError(t, err)
}
