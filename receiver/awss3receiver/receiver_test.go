// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awss3receiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awss3receiver"

import (
	"bytes"
	"compress/gzip"
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/ptrace"
	conventions "go.opentelemetry.io/collector/semconv/v1.22.0"
	"go.uber.org/zap"
)

func generateTraceData() ptrace.Traces {
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().PutStr(conventions.AttributeServiceName, "test")
	span := rs.ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span.SetSpanID([8]byte{0, 1, 2, 3, 4, 5, 6, 7})
	span.SetTraceID([16]byte{0, 1, 2, 3, 4, 5, 6, 7, 7, 6, 5, 4, 3, 2, 1, 0})
	span.SetStartTimestamp(1581452772000000000)
	span.SetEndTimestamp(1581452773000000000)
	return td
}

func gzipCompress(data []byte) []byte {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	_, _ = gz.Write(data)
	_ = gz.Close()
	return buf.Bytes()
}

func Test_receiveBytes(t *testing.T) {
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
			r := &awss3TraceReceiver{
				consumer: tracesConsumer,
				logger:   zap.NewNop(),
			}
			if err := r.receiveBytes(context.Background(), tt.args.key, tt.args.data); (err != nil) != tt.wantErr {
				t.Errorf("receiveBytes() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
