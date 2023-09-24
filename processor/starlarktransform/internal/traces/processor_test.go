package traces

import (
	"context"
	"errors"
	"testing"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

var testtraceevent = `{"resourceSpans":[{"resource":{"attributes":[{"key":"telemetry.sdk.language","value":{"stringValue":"python"}},{"key":"telemetry.sdk.name","value":{"stringValue":"opentelemetry"}},{"key":"telemetry.sdk.version","value":{"stringValue":"1.19.0"}},{"key":"telemetry.auto.version","value":{"stringValue":"0.40b0"}},{"key":"service.name","value":{"stringValue":"unknown_service"}}]},"scopeSpans":[{"scope":{"name":"opentelemetry.instrumentation.flask","version":"0.40b0"},"spans":[{"traceId":"9cb5bf738137b2248dc7b20445ec2e1c","spanId":"88079ad5c94b5b13","parentSpanId":"","name":"/roll","kind":2,"startTimeUnixNano":"1694388218052842000","endTimeUnixNano":"1694388218053415000","attributes":[{"key":"http.method","value":{"stringValue":"GET"}},{"key":"http.server_name","value":{"stringValue":"0.0.0.0"}},{"key":"http.scheme","value":{"stringValue":"http"}},{"key":"net.host.port","value":{"intValue":"5001"}},{"key":"http.host","value":{"stringValue":"localhost:5001"}},{"key":"http.target","value":{"stringValue":"/roll"}},{"key":"net.peer.ip","value":{"stringValue":"127.0.0.1"}},{"key":"http.user_agent","value":{"stringValue":"curl/7.87.0"}},{"key":"net.peer.port","value":{"intValue":"52365"}},{"key":"http.flavor","value":{"stringValue":"1.1"}},{"key":"http.route","value":{"stringValue":"/roll"}},{"key":"http.status_code","value":{"intValue":"200"}}],"status":{}}]}]}]}`

type fakeTraceConsumer struct {
	ptrace.JSONMarshaler
	t        *testing.T
	expected string
}

func (f *fakeTraceConsumer) ConsumeTraces(_ context.Context, md ptrace.Traces) error {
	if f.t == nil {
		return nil
	}

	b, err := f.MarshalTraces(md)
	require.NoError(f.t, err)
	assert.Equal(f.t, f.expected, string(b))
	return nil
}

func (f *fakeTraceConsumer) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func TestTraceConsumer(t *testing.T) {
	mp := NewProcessor(context.Background(),
		zap.NewNop(), "",
		&fakeTraceConsumer{t: t, expected: testtraceevent})

	testcases := []struct {
		name             string
		event            string
		code             string
		expectError      error
		expectStartError error
		next             consumer.Traces
	}{
		{
			name:  "update event input",
			event: testtraceevent,
			code: heredoc.Doc(`
			def transform(event):
				event = json.decode(event)
				for td in event['resourceSpans']:
					# add resource attribute
					td['resource']['attributes'].append({
						'key': 'source',
						'value': {
							'stringValue': 'starlarktransform'
						}
					})

				return event`),

			next: &fakeTraceConsumer{
				t:        t,
				expected: `{"resourceSpans":[{"resource":{"attributes":[{"key":"telemetry.sdk.language","value":{"stringValue":"python"}},{"key":"telemetry.sdk.name","value":{"stringValue":"opentelemetry"}},{"key":"telemetry.sdk.version","value":{"stringValue":"1.19.0"}},{"key":"telemetry.auto.version","value":{"stringValue":"0.40b0"}},{"key":"service.name","value":{"stringValue":"unknown_service"}},{"key":"source","value":{"stringValue":"starlarktransform"}}]},"scopeSpans":[{"scope":{"name":"opentelemetry.instrumentation.flask","version":"0.40b0"},"spans":[{"traceId":"9cb5bf738137b2248dc7b20445ec2e1c","spanId":"88079ad5c94b5b13","parentSpanId":"","name":"/roll","kind":2,"startTimeUnixNano":"1694388218052842000","endTimeUnixNano":"1694388218053415000","attributes":[{"key":"http.method","value":{"stringValue":"GET"}},{"key":"http.server_name","value":{"stringValue":"0.0.0.0"}},{"key":"http.scheme","value":{"stringValue":"http"}},{"key":"net.host.port","value":{"intValue":"5001"}},{"key":"http.host","value":{"stringValue":"localhost:5001"}},{"key":"http.target","value":{"stringValue":"/roll"}},{"key":"net.peer.ip","value":{"stringValue":"127.0.0.1"}},{"key":"http.user_agent","value":{"stringValue":"curl/7.87.0"}},{"key":"net.peer.port","value":{"intValue":"52365"}},{"key":"http.flavor","value":{"stringValue":"1.1"}},{"key":"http.route","value":{"stringValue":"/roll"}},{"key":"http.status_code","value":{"intValue":"200"}}],"status":{}}]}]}]}`,
			},
		},
		{
			name:  "nil or empty transform return",
			event: testtraceevent,
			code:  `def transform(event): return`,
			next:  &fakeTraceConsumer{t: t, expected: testtraceevent},
		},
		{
			name:        "bad transform syntax",
			event:       testtraceevent,
			code:        `def transform(event): event["cats"]; return`,
			next:        &fakeTraceConsumer{t: t, expected: testtraceevent},
			expectError: errors.New("error calling transform function:"),
		},
		{
			name:             "missing transform function",
			event:            testtraceevent,
			code:             `def run(event): return event`,
			next:             &fakeTraceConsumer{t: t, expected: testtraceevent},
			expectError:      nil,
			expectStartError: errors.New("starlark: no 'transform' function defined in script"),
		},
	}

	for _, tt := range testcases {
		t.Run(tt.name, func(t *testing.T) {
			mp.code = tt.code
			err := mp.Start(context.Background(), nil)
			if tt.expectStartError != nil {
				require.ErrorContains(t, err, tt.expectStartError.Error())
				return
			}

			require.NoError(t, err, nil)

			md, err := (&ptrace.JSONUnmarshaler{}).UnmarshalTraces([]byte(tt.event))
			require.NoError(t, err)

			mp.next = tt.next
			err = mp.ConsumeTraces(context.Background(), md)
			if tt.expectError != nil {
				require.ErrorContains(t, err, tt.expectError.Error())
				return
			}
			require.Equal(t, tt.expectError, err)
		})
	}
}

func BenchmarkLogConsumer(b *testing.B) {
	benchcases := []struct {
		name        string
		event       string
		code        string
		expectError error
		next        consumer.Traces
	}{
		{
			name:  "update event input",
			event: testtraceevent,
			code: heredoc.Doc(`
				def transform(event):
					event = json.decode(event)
					for md in event['resourceMetrics']:
						# prefix each metric name with starlarktransform
						for sm in md['scopeMetrics']:
							for m in sm['metrics']:
								m['name'] = 'starlarktransform.' + m['name']
		
					return event`),

			next: &fakeTraceConsumer{
				t:        nil,
				expected: `{"resourceLogs":[{"resource":{"attributes":[{"key":"log.file.name","value":{"stringValue":"other.log"}}]},"scopeLogs":[{"scope":{},"logRecords":[{"observedTimeUnixNano":"1694127596456358000","body":{"stringValue":"2023-09-06T01:09:24.045+0200    INFO    internal/command.go:117 OpenTelemetry Collector Builder {\"version\": \"0.84.0\", \"date\": \"2023-08-29T18:58:24Z\"}"},"attributes":[{"key":"app","value":{"stringValue":"dev"}}],"traceId":"","spanId":""}]}]}]}`,
			},
			expectError: nil,
		},
	}

	for _, bc := range benchcases {
		b.Run(bc.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				mp := NewProcessor(context.Background(),
					zap.NewNop(), "",
					bc.next)

				mp.code = bc.code
				require.NoError(b, mp.Start(context.Background(), nil))

				md, err := (&ptrace.JSONUnmarshaler{}).UnmarshalTraces([]byte(bc.event))
				if err != nil {
					b.Error(err)
				}

				if err = mp.ConsumeTraces(context.Background(), md); err != nil {
					b.Error(err)
				}

			}
		})
	}
}
