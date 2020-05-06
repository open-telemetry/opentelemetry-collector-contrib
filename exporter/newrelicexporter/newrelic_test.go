// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package newrelicexporter

import (
	"context"
	"testing"

	commonpb "github.com/census-instrumentation/opencensus-proto/gen-go/agent/common/v1"
	resourcepb "github.com/census-instrumentation/opencensus-proto/gen-go/resource/v1"
	tracepb "github.com/census-instrumentation/opencensus-proto/gen-go/trace/v1"
	"github.com/open-telemetry/opentelemetry-collector/consumer/consumerdata"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func TestLogWriter(t *testing.T) {
	var messages []string
	l := logWriter{func(s string, _ ...zapcore.Field) {
		messages = append(messages, s)
	}}

	n, err := l.Write([]byte("one"))
	require.NoError(t, err)
	assert.Equal(t, 3, n)
	assert.Len(t, messages, 1)

	n, err = l.Write([]byte("two"))
	require.NoError(t, err)
	assert.Equal(t, 3, n)
	assert.Len(t, messages, 2)
}

func TestExportTraceData(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	m := &Mock{make([]Data, 0, 3)}
	srv := m.Server()
	defer srv.Close()

	td := consumerdata.TraceData{
		Node: &commonpb.Node{
			ServiceInfo: &commonpb.ServiceInfo{Name: "test-service"},
		},
		Resource: &resourcepb.Resource{
			Labels: map[string]string{
				"resource": "R1",
			},
		},
		Spans: []*tracepb.Span{
			{
				TraceId: []byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1},
				SpanId:  []byte{0, 0, 0, 0, 0, 0, 0, 1},
				Name:    &tracepb.TruncatableString{Value: "root"},
				Status:  &tracepb.Status{},
			},
			{
				TraceId:      []byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1},
				SpanId:       []byte{0, 0, 0, 0, 0, 0, 0, 2},
				ParentSpanId: []byte{0, 0, 0, 0, 0, 0, 0, 1},
				Name:         &tracepb.TruncatableString{Value: "client"},
				Status:       &tracepb.Status{},
			},
			{
				TraceId:      []byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1},
				SpanId:       []byte{0, 0, 0, 0, 0, 0, 0, 3},
				ParentSpanId: []byte{0, 0, 0, 0, 0, 0, 0, 2},
				Name:         &tracepb.TruncatableString{Value: "server"},
				Status:       &tracepb.Status{},
			},
		},
	}

	f := new(Factory)
	c := f.CreateDefaultConfig().(*Config)
	c.APIKey, c.SpansURLOverride = "1", srv.URL
	l := zap.NewNop()
	exp, err := f.CreateTraceExporter(l, c)
	require.NoError(t, err)
	require.NoError(t, exp.ConsumeTraceData(ctx, td))
	require.NoError(t, exp.Shutdown(ctx))

	expected := []Span{
		{
			ID:      "0000000000000001",
			TraceID: "01010101010101010101010101010101",
			Attributes: map[string]interface{}{
				"collector.name":    name,
				"collector.version": version,
				"name":              "root",
				"resource":          "R1",
				"service.name":      "test-service",
			},
		},
		{
			ID:      "0000000000000002",
			TraceID: "01010101010101010101010101010101",
			Attributes: map[string]interface{}{
				"collector.name":    name,
				"collector.version": version,
				"name":              "client",
				"parent.id":         "0000000000000001",
				"resource":          "R1",
				"service.name":      "test-service",
			},
		},
		{
			ID:      "0000000000000003",
			TraceID: "01010101010101010101010101010101",
			Attributes: map[string]interface{}{
				"collector.name":    name,
				"collector.version": version,
				"name":              "server",
				"parent.id":         "0000000000000002",
				"resource":          "R1",
				"service.name":      "test-service",
			},
		},
	}

	//assert.ElementsMatch(t, expected, m.Spans())
	assert.Equal(t, expected, m.Spans())
}
