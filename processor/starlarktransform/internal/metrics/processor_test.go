package metrics

import (
	"context"
	"errors"
	"testing"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

var testmetricevent = `{"resourceMetrics":[{"resource":{},"scopeMetrics":[{"scope":{"name":"otelcol/hostmetricsreceiver/memory","version":"0.84.0"},"metrics":[{"name":"system.memory.usage","description":"Bytes of memory in use.","unit":"By","sum":{"dataPoints":[{"attributes":[{"key":"state","value":{"stringValue":"used"}}],"startTimeUnixNano":"1694171569000000000","timeUnixNano":"1694189699786689531","asInt":"1874247680"},{"attributes":[{"key":"state","value":{"stringValue":"free"}}],"startTimeUnixNano":"1694171569000000000","timeUnixNano":"1694189699786689531","asInt":"29214199808"}],"aggregationTemporality":2}}]}],"schemaUrl":"https://opentelemetry.io/schemas/1.9.0"}]}`

type fakeMetricConsumer struct {
	pmetric.JSONMarshaler
	t        *testing.T
	expected string
}

func (f *fakeMetricConsumer) ConsumeMetrics(_ context.Context, md pmetric.Metrics) error {
	if f.t == nil {
		return nil
	}

	b, err := f.MarshalMetrics(md)
	require.NoError(f.t, err)
	assert.Equal(f.t, f.expected, string(b))
	return nil
}

func (f *fakeMetricConsumer) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func TestMetricConsumer(t *testing.T) {
	mp := NewProcessor(context.Background(),
		zap.NewNop(), "",
		&fakeMetricConsumer{t: t, expected: testmetricevent})

	testcases := []struct {
		name             string
		event            string
		code             string
		expectError      error
		expectStartError error
		next             consumer.Metrics
	}{
		{
			name:  "update event input",
			event: testmetricevent,
			code: heredoc.Doc(`
				def transform(event):
					event = json.decode(event)
					for md in event['resourceMetrics']:
						# prefix each metric name with starlarktransform
						for sm in md['scopeMetrics']:
							for m in sm['metrics']:
								m['name'] = 'starlarktransform.' + m['name']
		
					return event`),

			next: &fakeMetricConsumer{
				t:        t,
				expected: `{"resourceMetrics":[{"resource":{},"scopeMetrics":[{"scope":{"name":"otelcol/hostmetricsreceiver/memory","version":"0.84.0"},"metrics":[{"name":"starlarktransform.system.memory.usage","description":"Bytes of memory in use.","unit":"By","sum":{"dataPoints":[{"attributes":[{"key":"state","value":{"stringValue":"used"}}],"startTimeUnixNano":"1694171569000000000","timeUnixNano":"1694189699786689531","asInt":"1874247680"},{"attributes":[{"key":"state","value":{"stringValue":"free"}}],"startTimeUnixNano":"1694171569000000000","timeUnixNano":"1694189699786689531","asInt":"29214199808"}],"aggregationTemporality":2}}]}],"schemaUrl":"https://opentelemetry.io/schemas/1.9.0"}]}`,
			},
		},
		{
			name:  "nil or empty transform return",
			event: testmetricevent,
			code:  `def transform(event): return`,
			next:  &fakeMetricConsumer{t: t, expected: testmetricevent},
		},
		{
			name:        "bad transform syntax",
			event:       testmetricevent,
			code:        `def transform(event): event["cats"]; return`,
			next:        &fakeMetricConsumer{t: t, expected: testmetricevent},
			expectError: errors.New("error calling transform function:"),
		},
		{
			name:             "missing transform function",
			event:            testmetricevent,
			code:             `def run(event): return event`,
			next:             &fakeMetricConsumer{t: t, expected: testmetricevent},
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

			md, err := (&pmetric.JSONUnmarshaler{}).UnmarshalMetrics([]byte(tt.event))
			require.NoError(t, err)

			mp.next = tt.next
			err = mp.ConsumeMetrics(context.Background(), md)
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
		next        consumer.Metrics
	}{
		{
			name:  "update event input",
			event: testmetricevent,
			code: heredoc.Doc(`
				def transform(event):
					event = json.decode(event)
					for md in event['resourceMetrics']:
						# prefix each metric name with starlarktransform
						for sm in md['scopeMetrics']:
							for m in sm['metrics']:
								m['name'] = 'starlarktransform.' + m['name']
		
					return event`),

			next: &fakeMetricConsumer{
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

				md, err := (&pmetric.JSONUnmarshaler{}).UnmarshalMetrics([]byte(bc.event))
				if err != nil {
					b.Error(err)
				}

				if err = mp.ConsumeMetrics(context.Background(), md); err != nil {
					b.Error(err)
				}

			}
		})
	}
}
