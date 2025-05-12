// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awss3receiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awss3receiver"

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
	"go.opentelemetry.io/collector/receiver/receivertest"
	conventions "go.opentelemetry.io/otel/semconv/v1.22.0"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awss3receiver/internal/metadata"
)

func generateTraceData() ptrace.Traces {
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().PutStr(string(conventions.ServiceNameKey), "test")
	span := rs.ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span.SetSpanID([8]byte{0, 1, 2, 3, 4, 5, 6, 7})
	span.SetTraceID([16]byte{0, 1, 2, 3, 4, 5, 6, 7, 7, 6, 5, 4, 3, 2, 1, 0})
	span.SetStartTimestamp(1581452772000000000)
	span.SetEndTimestamp(1581452773000000000)
	return td
}

func generateMetricData() pmetric.Metrics {
	md := pmetric.NewMetrics()
	rs := md.ResourceMetrics().AppendEmpty()
	metric := rs.ScopeMetrics().AppendEmpty().Metrics()
	dp := metric.AppendEmpty().SetEmptyGauge().DataPoints().AppendEmpty()
	dp.SetDoubleValue(1.0)
	dp.SetStartTimestamp(1581452772000000000)
	dp.SetTimestamp(1581452772000000000)
	return md
}

func generateLogData() plog.Logs {
	ld := plog.NewLogs()
	rs := ld.ResourceLogs().AppendEmpty()
	log := rs.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
	log.SetObservedTimestamp(1581452772000000000)
	log.Body().SetStr("test")
	return ld
}

func gzipCompress(data []byte) []byte {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	_, _ = gz.Write(data)
	_ = gz.Close()
	return buf.Bytes()
}

type hostWithExtensions struct {
	extensions map[component.ID]component.Component
}

func (h hostWithExtensions) GetExtensions() map[component.ID]component.Component {
	return h.extensions
}

type nonEncodingExtension struct{}

func (e nonEncodingExtension) Start(_ context.Context, _ component.Host) error {
	return nil
}

func (e nonEncodingExtension) Shutdown(_ context.Context) error {
	return nil
}

type unmarshalExtension struct {
	trace  ptrace.Traces
	metric pmetric.Metrics
	log    plog.Logs
}

func (e unmarshalExtension) Start(_ context.Context, _ component.Host) error {
	return nil
}

func (e unmarshalExtension) Shutdown(_ context.Context) error {
	return nil
}

func (e unmarshalExtension) UnmarshalTraces(_ []byte) (ptrace.Traces, error) {
	return e.trace, nil
}

func (e unmarshalExtension) UnmarshalMetrics(_ []byte) (pmetric.Metrics, error) {
	return e.metric, nil
}

func (e unmarshalExtension) UnmarshalLogs(_ []byte) (plog.Logs, error) {
	return e.log, nil
}

func Test_receiveBytes_traces(t *testing.T) {
	testTrace := generateTraceData()

	jsonTrace, err := (&ptrace.JSONMarshaler{}).MarshalTraces(testTrace)
	require.NoError(t, err)
	protobufTrace, err := (&ptrace.ProtoMarshaler{}).MarshalTraces(testTrace)
	require.NoError(t, err)

	type args struct {
		key  string
		data []byte
	}
	tests := []struct {
		name      string
		args      args
		wantErr   bool
		wantTrace bool
	}{
		{
			name: "nil data",
			args: args{
				key:  "test.json",
				data: nil,
			},
			wantErr:   false,
			wantTrace: false,
		},
		{
			name: ".json",
			args: args{
				key:  "test.json",
				data: jsonTrace,
			},
			wantErr:   false,
			wantTrace: true,
		},
		{
			name: ".binpb",
			args: args{
				key:  "test.binpb",
				data: protobufTrace,
			},
			wantErr:   false,
			wantTrace: true,
		},
		{
			name: ".unknown",
			args: args{
				key:  "test.unknown",
				data: []byte("unknown"),
			},
			wantErr:   false,
			wantTrace: false,
		},
		{
			name: ".json.gz",
			args: args{
				key:  "test.json.gz",
				data: gzipCompress(jsonTrace),
			},
			wantErr:   false,
			wantTrace: true,
		},
		{
			name: ".binpb.gz",
			args: args{
				key:  "test.binpb.gz",
				data: gzipCompress(protobufTrace),
			},
			wantErr:   false,
			wantTrace: true,
		},
		{
			name: "encoding extension",
			args: args{
				key:  "test.test",
				data: []byte("test"),
			},
			wantErr:   false,
			wantTrace: true,
		},
		{
			name: "encoding extension .gz",
			args: args{
				key:  "test.test",
				data: gzipCompress([]byte("test")),
			},
			wantErr:   false,
			wantTrace: true,
		},
		{
			name: "invalid gzip",
			args: args{
				key:  "test.json.gz",
				data: []byte("invalid gzip"),
			},
			wantErr:   true,
			wantTrace: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tracesConsumer, _ := consumer.NewTraces(func(_ context.Context, td ptrace.Traces) error {
				t.Helper()
				if !tt.wantTrace {
					t.Errorf("receiveBytes() received unexpected trace")
				} else {
					require.Equal(t, testTrace, td)
				}
				return nil
			})
			obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{ReceiverCreateSettings: receivertest.NewNopSettings(metadata.Type)})
			require.NoError(t, err)
			r := &awss3Receiver{
				logger:  zap.NewNop(),
				obsrecv: obsrecv,
				extensions: encodingExtensions{
					{
						extension: &unmarshalExtension{trace: testTrace},
						suffix:    ".test",
					},
				},
				dataProcessor: &traceReceiver{
					consumer: tracesConsumer,
				},
			}
			if err := r.receiveBytes(context.Background(), tt.args.key, tt.args.data); (err != nil) != tt.wantErr {
				t.Errorf("receiveBytes() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_receiveBytes_metrics(t *testing.T) {
	testMetric := generateMetricData()

	jsonMetric, err := (&pmetric.JSONMarshaler{}).MarshalMetrics(testMetric)
	require.NoError(t, err)
	protobufMetric, err := (&pmetric.ProtoMarshaler{}).MarshalMetrics(testMetric)
	require.NoError(t, err)

	type args struct {
		key  string
		data []byte
	}
	tests := []struct {
		name       string
		args       args
		wantErr    bool
		wantMetric bool
	}{
		{
			name: "nil data",
			args: args{
				key:  "test.json",
				data: nil,
			},
			wantErr:    false,
			wantMetric: false,
		},
		{
			name: ".json",
			args: args{
				key:  "test.json",
				data: jsonMetric,
			},
			wantErr:    false,
			wantMetric: true,
		},
		{
			name: ".binpb",
			args: args{
				key:  "test.binpb",
				data: protobufMetric,
			},
			wantErr:    false,
			wantMetric: true,
		},
		{
			name: ".unknown",
			args: args{
				key:  "test.unknown",
				data: []byte("unknown"),
			},
			wantErr:    false,
			wantMetric: false,
		},
		{
			name: ".json.gz",
			args: args{
				key:  "test.json.gz",
				data: gzipCompress(jsonMetric),
			},
			wantErr:    false,
			wantMetric: true,
		},
		{
			name: ".binpb.gz",
			args: args{
				key:  "test.binpb.gz",
				data: gzipCompress(protobufMetric),
			},
			wantErr:    false,
			wantMetric: true,
		},
		{
			name: "encoding extension",
			args: args{
				key:  "test.test",
				data: []byte("test"),
			},
			wantErr:    false,
			wantMetric: true,
		},
		{
			name: "encoding extension .gz",
			args: args{
				key:  "test.test",
				data: gzipCompress([]byte("test")),
			},
			wantErr:    false,
			wantMetric: true,
		},
		{
			name: "invalid gzip",
			args: args{
				key:  "test.json.gz",
				data: []byte("invalid gzip"),
			},
			wantErr:    true,
			wantMetric: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tracesConsumer, _ := consumer.NewMetrics(func(_ context.Context, md pmetric.Metrics) error {
				t.Helper()
				if !tt.wantMetric {
					t.Errorf("receiveBytes() received unexpected trace")
				} else {
					require.Equal(t, testMetric, md)
				}
				return nil
			})
			obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{ReceiverCreateSettings: receivertest.NewNopSettings(metadata.Type)})
			require.NoError(t, err)
			r := &awss3Receiver{
				logger:  zap.NewNop(),
				obsrecv: obsrecv,
				extensions: encodingExtensions{
					{
						extension: &unmarshalExtension{metric: testMetric},
						suffix:    ".test",
					},
				},
				dataProcessor: &metricsReceiver{
					consumer: tracesConsumer,
				},
			}
			if err := r.receiveBytes(context.Background(), tt.args.key, tt.args.data); (err != nil) != tt.wantErr {
				t.Errorf("receiveBytes() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_receiveBytes_logs(t *testing.T) {
	testLog := generateLogData()

	jsonLog, err := (&plog.JSONMarshaler{}).MarshalLogs(testLog)
	require.NoError(t, err)
	protobufLog, err := (&plog.ProtoMarshaler{}).MarshalLogs(testLog)
	require.NoError(t, err)

	type args struct {
		key  string
		data []byte
	}
	tests := []struct {
		name       string
		args       args
		wantErr    bool
		wantMetric bool
	}{
		{
			name: "nil data",
			args: args{
				key:  "test.json",
				data: nil,
			},
			wantErr:    false,
			wantMetric: false,
		},
		{
			name: ".json",
			args: args{
				key:  "test.json",
				data: jsonLog,
			},
			wantErr:    false,
			wantMetric: true,
		},
		{
			name: ".binpb",
			args: args{
				key:  "test.binpb",
				data: protobufLog,
			},
			wantErr:    false,
			wantMetric: true,
		},
		{
			name: ".unknown",
			args: args{
				key:  "test.unknown",
				data: []byte("unknown"),
			},
			wantErr:    false,
			wantMetric: false,
		},
		{
			name: ".json.gz",
			args: args{
				key:  "test.json.gz",
				data: gzipCompress(jsonLog),
			},
			wantErr:    false,
			wantMetric: true,
		},
		{
			name: ".binpb.gz",
			args: args{
				key:  "test.binpb.gz",
				data: gzipCompress(protobufLog),
			},
			wantErr:    false,
			wantMetric: true,
		},
		{
			name: "encoding extension",
			args: args{
				key:  "test.test",
				data: []byte("test"),
			},
			wantErr:    false,
			wantMetric: true,
		},
		{
			name: "encoding extension .gz",
			args: args{
				key:  "test.test",
				data: gzipCompress([]byte("test")),
			},
			wantErr:    false,
			wantMetric: true,
		},
		{
			name: "invalid gzip",
			args: args{
				key:  "test.json.gz",
				data: []byte("invalid gzip"),
			},
			wantErr:    true,
			wantMetric: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tracesConsumer, _ := consumer.NewLogs(func(_ context.Context, ld plog.Logs) error {
				t.Helper()
				if !tt.wantMetric {
					t.Errorf("receiveBytes() received unexpected trace")
				} else {
					require.Equal(t, testLog, ld)
				}
				return nil
			})
			obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{ReceiverCreateSettings: receivertest.NewNopSettings(metadata.Type)})
			require.NoError(t, err)
			r := &awss3Receiver{
				logger:  zap.NewNop(),
				obsrecv: obsrecv,
				extensions: encodingExtensions{
					{
						extension: &unmarshalExtension{log: testLog},
						suffix:    ".test",
					},
				},
				dataProcessor: &logsReceiver{
					consumer: tracesConsumer,
				},
			}
			if err := r.receiveBytes(context.Background(), tt.args.key, tt.args.data); (err != nil) != tt.wantErr {
				t.Errorf("receiveBytes() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_newEncodingExtensions(t *testing.T) {
	expectedExtension := &unmarshalExtension{}
	host := hostWithExtensions{
		extensions: map[component.ID]component.Component{
			component.MustNewID("encoding"):    expectedExtension,
			component.MustNewID("nonencoding"): &nonEncodingExtension{},
		},
	}
	type args struct {
		encodingsConfig []Encoding
		host            component.Host
	}
	tests := []struct {
		name    string
		args    args
		want    encodingExtensions
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "encoding",
			args: args{
				encodingsConfig: []Encoding{
					{
						Extension: component.MustNewID("encoding"),
						Suffix:    ".encoding",
					},
				},
				host: host,
			},
			want: encodingExtensions{
				encodingExtension{
					extension: expectedExtension,
					suffix:    ".encoding",
				},
			},
			wantErr: assert.NoError,
		},
		{
			name: "empty",
			args: args{
				encodingsConfig: []Encoding{},
				host:            host,
			},
			want:    encodingExtensions{},
			wantErr: assert.NoError,
		},
		{
			name: "missing extension",
			args: args{
				encodingsConfig: []Encoding{
					{
						Extension: component.MustNewID("missing"),
						Suffix:    ".missing",
					},
				},
				host: host,
			},
			want:    nil,
			wantErr: assert.Error,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := newEncodingExtensions(tt.args.encodingsConfig, tt.args.host)
			if !tt.wantErr(t, err, fmt.Sprintf("newEncodingExtensions(%v, %v)", tt.args.encodingsConfig, tt.args.host)) {
				return
			}
			assert.Equalf(t, tt.want, got, "newEncodingExtensions(%v, %v)", tt.args.encodingsConfig, tt.args.host)
		})
	}
}

func Test_encodingExtensions_findExtension(t *testing.T) {
	type args struct {
		key string
	}
	tests := []struct {
		name      string
		encodings encodingExtensions
		args      args
		want      ptrace.Unmarshaler
		want1     string
	}{
		{
			name:      "empty",
			encodings: encodingExtensions{},
			args: args{
				key: "test",
			},
			want:  nil,
			want1: "",
		},
		{
			name: "found",
			encodings: encodingExtensions{
				{
					extension: &unmarshalExtension{},
					suffix:    ".test",
				},
			},
			args: args{
				key: "test.test",
			},
			want:  &unmarshalExtension{},
			want1: ".test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := tt.encodings.findExtension(tt.args.key)
			assert.Equalf(t, tt.want, got, "findExtension(%v)", tt.args.key)
			assert.Equalf(t, tt.want1, got1, "findExtension(%v)", tt.args.key)
		})
	}
}
