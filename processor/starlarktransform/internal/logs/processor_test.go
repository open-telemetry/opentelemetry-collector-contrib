package logs

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"
)

var testlogevent = `{"resourceLogs":[{"resource":{"attributes":[{"key":"log.file.name","value":{"stringValue":"test.log"}}]},"scopeLogs":[{"scope":{},"logRecords":[{"observedTimeUnixNano":"1694127596456358000","body":{"stringValue":"2023-09-06T01:09:24.045+0200    INFO    internal/command.go:117 OpenTelemetry Collector Builder {\"version\": \"0.84.0\", \"date\": \"2023-08-29T18:58:24Z\"}"},"attributes":[{"key":"app","value":{"stringValue":"dev"}}],"traceId":"","spanId":""}]}]}]}`

type fakeLogConsumer struct {
	plog.JSONMarshaler
	t        *testing.T
	expected string
}

func (f *fakeLogConsumer) ConsumeLogs(_ context.Context, ld plog.Logs) error {
	if f.t == nil {
		_ = ld // for benchmarking
		return nil
	}

	b, err := f.MarshalLogs(ld)
	require.NoError(f.t, err)
	assert.Equal(f.t, f.expected, string(b))
	return nil
}

func (f *fakeLogConsumer) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func TestLogConsumer(t *testing.T) {
	lp := NewProcessor(context.Background(),
		zap.NewNop(), "",
		&fakeLogConsumer{t: t, expected: testlogevent})

	testcases := []struct {
		name             string
		event            string
		code             string
		expectError      error
		expectStartError error
		next             consumer.Logs
	}{
		{
			name:  "update event input",
			event: testlogevent,
			code: heredoc.Doc(`
						def transform(event):
							e = json.decode(event)
							e["resourceLogs"][0]["resource"]["attributes"][0]["value"]["stringValue"] = "other.log"
							return e`),

			next: &fakeLogConsumer{
				t:        t,
				expected: `{"resourceLogs":[{"resource":{"attributes":[{"key":"log.file.name","value":{"stringValue":"other.log"}}]},"scopeLogs":[{"scope":{},"logRecords":[{"observedTimeUnixNano":"1694127596456358000","body":{"stringValue":"2023-09-06T01:09:24.045+0200    INFO    internal/command.go:117 OpenTelemetry Collector Builder {\"version\": \"0.84.0\", \"date\": \"2023-08-29T18:58:24Z\"}"},"attributes":[{"key":"app","value":{"stringValue":"dev"}}],"traceId":"","spanId":""}]}]}]}`,
			},
		},
		{
			name:  "nil or empty transform return",
			event: testlogevent,
			code:  `def transform(event): return`,
			next:  &fakeLogConsumer{t: t, expected: testlogevent},
		},
		{
			name:        "bad transform syntax",
			event:       testlogevent,
			code:        `def transform(event): event["cats"]; return`,
			next:        &fakeLogConsumer{t: t, expected: testlogevent},
			expectError: errors.New("error calling transform function:"),
		},
		{
			name:             "missing transform function",
			event:            testlogevent,
			code:             `def run(event): return event`,
			next:             &fakeLogConsumer{t: t, expected: testlogevent},
			expectError:      nil,
			expectStartError: errors.New("starlark: no 'transform' function defined in script"),
		},
	}

	for _, tt := range testcases {
		t.Run(tt.name, func(t *testing.T) {
			lp.code = tt.code
			err := lp.Start(context.Background(), nil)
			if tt.expectStartError != nil {
				require.ErrorContains(t, err, tt.expectStartError.Error())
				return
			}

			require.NoError(t, err, nil)

			ld, err := (&plog.JSONUnmarshaler{}).UnmarshalLogs([]byte(tt.event))
			require.NoError(t, err)

			lp.next = tt.next
			err = lp.ConsumeLogs(context.Background(), ld)
			if tt.expectError != nil {
				require.ErrorContains(t, err, tt.expectError.Error())
				return
			}
			require.Equal(t, tt.expectError, err)
		})
	}
}

var (
	TestLogTime      = time.Date(2020, 2, 11, 20, 26, 12, 321, time.UTC)
	TestLogTimestamp = pcommon.NewTimestampFromTime(TestLogTime)

	TestObservedTime      = time.Date(2020, 2, 11, 20, 26, 13, 789, time.UTC)
	TestObservedTimestamp = pcommon.NewTimestampFromTime(TestObservedTime)

	traceID = [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	spanID  = [8]byte{1, 2, 3, 4, 5, 6, 7, 8}
)

func constructLogs() plog.Logs {
	td := plog.NewLogs()
	rs0 := td.ResourceLogs().AppendEmpty()
	rs0.Resource().Attributes().PutStr("host.name", "localhost")
	rs0ils0 := rs0.ScopeLogs().AppendEmpty()
	rs0ils0.Scope().SetName("scope")
	fillLogOne(rs0ils0.LogRecords().AppendEmpty())
	fillLogTwo(rs0ils0.LogRecords().AppendEmpty())
	return td
}

func fillLogOne(log plog.LogRecord) {
	log.Body().SetStr("operationA")
	log.SetTimestamp(TestLogTimestamp)
	log.SetObservedTimestamp(TestObservedTimestamp)
	log.SetDroppedAttributesCount(1)
	log.SetFlags(plog.DefaultLogRecordFlags.WithIsSampled(true))
	log.SetSeverityNumber(1)
	log.SetTraceID(traceID)
	log.SetSpanID(spanID)
	log.Attributes().PutStr("http.method", "get")
	log.Attributes().PutStr("http.path", "/health")
	log.Attributes().PutStr("http.url", "http://localhost/health")
	log.Attributes().PutStr("flags", "A|B|C")
	log.Attributes().PutStr("total.string", "123456789")

}

func fillLogTwo(log plog.LogRecord) {
	log.Body().SetStr("operationB")
	log.SetTimestamp(TestLogTimestamp)
	log.SetObservedTimestamp(TestObservedTimestamp)
	log.Attributes().PutStr("http.method", "get")
	log.Attributes().PutStr("http.path", "/health")
	log.Attributes().PutStr("http.url", "http://localhost/health")
	log.Attributes().PutStr("flags", "C|D")
	log.Attributes().PutStr("total.string", "345678")

}

func BenchmarkLogProcessor(b *testing.B) {
	benchcases := []struct {
		name string
		code string
	}{
		{
			`set(attributes["test"], "pass") where body == "operationA"`,
			heredoc.Doc(`
				def transform(event):
					e = json.decode(event)
					for r in e["resourceLogs"]:
						for sl in r["scopeLogs"]:
							for lr in sl["logRecords"]:
								if lr["body"]["stringValue"] == "operationA":
									lr["attributes"].append({
										"key": "test",
										"value": {"stringValue": "pass"}
									})
					return e`),
		},
		{
			`set(attributes["test"], "pass") where resource.attributes["host.name"] == "localhost"`,
			heredoc.Doc(`
				def transform(event):
					e = json.decode(event)
					for rlogs in e["resourceLogs"]:
						if [
							r for r in rlogs['resource']['attributes']
							if r['key'] == 'host.name' and r['value']['stringValue'] == 'localhost'
						]:
							for sl in rlogs["scopeLogs"]:
								for lr in sl["logRecords"]:
									lr["attributes"].append({
										"key": "test",
										"value": {"stringValue": "pass"}
									})
					return e`),
		},
	}

	for _, bc := range benchcases {
		b.Run(bc.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				ld := constructLogs()
				lp := NewProcessor(context.Background(),
					zap.NewNop(), bc.code,
					&fakeLogConsumer{})

				if err := lp.Start(context.Background(), nil); err != nil {
					b.Error(err)
				}

				if err := lp.ConsumeLogs(context.Background(), ld); err != nil {
					b.Error(err)
				}
			}
		})
	}
}
