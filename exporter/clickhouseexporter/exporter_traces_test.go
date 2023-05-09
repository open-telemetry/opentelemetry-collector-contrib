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

package clickhouseexporter

import (
	"context"
	"database/sql/driver"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	conventions "go.opentelemetry.io/collector/semconv/v1.18.0"
	"go.uber.org/zap/zaptest"
)

func TestExporter_pushTracesData(t *testing.T) {
	t.Run("push success", func(t *testing.T) {
		var items int
		initClickhouseTestServer(t, func(query string, values []driver.Value) error {
			t.Logf("%d, values:%+v", items, values)
			if strings.HasPrefix(query, "INSERT") {
				items++
			}
			return nil
		})

		exporter := newTestTracesExporter(t, defaultEndpoint)
		mustPushTracesData(t, exporter, simpleTraces(1))
		mustPushTracesData(t, exporter, simpleTraces(2))

		require.Equal(t, 3, items)
	})
	t.Run("test scope name and scope version merge into span attribute", func(t *testing.T) {
		initClickhouseTestServer(t, func(query string, values []driver.Value) error {
			if strings.HasPrefix(query, "INSERT") {
				if value, ok := values[9].(map[string]string); ok {
					v, found := value[conventions.AttributeOtelScopeName]
					require.True(t, found)
					require.Equal(t, "io.opentelemetry.contrib.clickhouse", v)
					v, found = value[conventions.AttributeOtelScopeVersion]
					require.True(t, found)
					require.Equal(t, "1.0.0", v)
				}
			}
			return nil
		})

		exporter := newTestTracesExporter(t, defaultEndpoint)
		mustPushTracesData(t, exporter, simpleTraces(1))
	})
}

func newTestTracesExporter(t *testing.T, dsn string, fns ...func(*Config)) *tracesExporter {
	exporter, err := newTracesExporter(zaptest.NewLogger(t), withTestExporterConfig(fns...)(dsn))
	require.NoError(t, err)
	require.NoError(t, exporter.start(context.TODO(), nil))

	t.Cleanup(func() { _ = exporter.shutdown(context.TODO()) })
	return exporter
}

func simpleTraces(count int) ptrace.Traces {
	traces := ptrace.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()
	ss.Scope().SetName("io.opentelemetry.contrib.clickhouse")
	ss.Scope().SetVersion("1.0.0")
	for i := 0; i < count; i++ {
		s := ss.Spans().AppendEmpty()
		s.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		s.SetEndTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		s.Attributes().PutStr(conventions.AttributeServiceName, "v")
		event := s.Events().AppendEmpty()
		event.SetName("event1")
		event.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		link := s.Links().AppendEmpty()
		link.Attributes().PutStr("k", "v")
	}
	return traces
}

func mustPushTracesData(t *testing.T, exporter *tracesExporter, td ptrace.Traces) {
	err := exporter.pushTraceData(context.TODO(), td)
	require.NoError(t, err)
}
