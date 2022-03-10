// Copyright  The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package schemaprocessor

import (
	"context"
	_ "embed"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap/zaptest"
)

//go:embed testdata/schema.yml
var schemaContent []byte

func SchemaHandler(t *testing.T) func(wr http.ResponseWriter, r *http.Request) {
	assert.NotEmpty(t, schemaContent, "SchemaContent MUST not be empty")
	return func(wr http.ResponseWriter, r *http.Request) {
		_, err := wr.Write(schemaContent)
		assert.NoError(t, err, "Must not have issues writing schema content")
	}
}

func newTestTransformer(t *testing.T) *transformer {
	trans, err := newTransformer(context.Background(), NewDefaultConfiguration(), component.ProcessorCreateSettings{
		TelemetrySettings: component.TelemetrySettings{
			Logger: zaptest.NewLogger(t),
		},
	})
	require.NoError(t, err, "Must not error when creating default transformer")
	return trans
}

func TestTransformerStart(t *testing.T) {
	t.Parallel()

	trans := newTestTransformer(t)
	assert.NoError(t, trans.start(context.Background(), nil))
}

func TestTrasnformerProcessing(t *testing.T) {
	t.Parallel()

	trans := newTestTransformer(t)
	t.Run("metrics", func(t *testing.T) {
		in := pdata.NewMetrics()
		in.ResourceMetrics().AppendEmpty()
		in.ResourceMetrics().At(0).SetSchemaUrl("http://opentelemetry.io/schames/1.9.0")
		in.ResourceMetrics().At(0).InstrumentationLibraryMetrics().AppendEmpty()
		m := in.ResourceMetrics().At(0).InstrumentationLibraryMetrics().At(0).Metrics().AppendEmpty()
		m.SetName("test-data")
		m.SetDescription("Only used throughout tests")
		m.SetUnit("seconds")
		m.CopyTo(in.ResourceMetrics().At(0).InstrumentationLibraryMetrics().At(0).Metrics().At(0))

		out, err := trans.processMetrics(context.Background(), in)
		assert.NoError(t, err, "Must not error when processing metrics")
		assert.Equal(t, in, out, "Must return the same data (subject to change)")
	})

	t.Run("traces", func(t *testing.T) {
		in := pdata.NewTraces()
		in.ResourceSpans().AppendEmpty()
		in.ResourceSpans().At(0).SetSchemaUrl("http://opentelemetry.io/schames/1.9.0")
		in.ResourceSpans().At(0).InstrumentationLibrarySpans().AppendEmpty()
		s := in.ResourceSpans().At(0).InstrumentationLibrarySpans().At(0).Spans().AppendEmpty()
		s.SetName("http.request")
		s.SetKind(pdata.SpanKindConsumer)
		s.SetSpanID(pdata.NewSpanID([8]byte{0, 1, 2, 3, 4, 5, 6, 7}))
		s.SetTraceState(pdata.TraceStateEmpty)
		s.CopyTo(in.ResourceSpans().At(0).InstrumentationLibrarySpans().At(0).Spans().At(0))

		out, err := trans.processTraces(context.Background(), in)
		assert.NoError(t, err, "Must not error when processing metrics")
		assert.Equal(t, in, out, "Must return the same data (subject to change)")
	})

	t.Run("logs", func(t *testing.T) {
		in := pdata.NewLogs()
		in.ResourceLogs().AppendEmpty()
		in.ResourceLogs().At(0).SetSchemaUrl("http://opentelemetry.io/schames/1.9.0")
		in.ResourceLogs().At(0).InstrumentationLibraryLogs().AppendEmpty()
		l := in.ResourceLogs().At(0).InstrumentationLibraryLogs().At(0).InstrumentationLibrary()
		l.SetName("magical-logs")
		l.SetVersion("alpha")
		l.CopyTo(in.ResourceLogs().At(0).InstrumentationLibraryLogs().At(0).InstrumentationLibrary())

		out, err := trans.processLogs(context.Background(), in)
		assert.NoError(t, err, "Must not error when processing metrics")
		assert.Equal(t, in, out, "Must return the same data (subject to change)")
	})
}
