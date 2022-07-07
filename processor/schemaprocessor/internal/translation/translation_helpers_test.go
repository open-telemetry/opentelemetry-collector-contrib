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
package translation

import (
	"bytes"
	"embed"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

const (
	TranslationVersion190 = "1.9.0"
	TranslationVersion161 = "1.6.1"

	prefix = "testdata"
)

//go:embed testdata
var testdataFiles embed.FS

func NewTranslationServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Rewrite request path so it doesn't require
		p := path.Join(prefix, r.URL.Path)
		f, err := testdataFiles.Open(p)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
		}
		if _, err := io.Copy(w, f); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		// While strictly not needed since embed uses a NopCloser
		// still worth keeping up with good practices
		_ = f.Close()
	}))
}

func LoadTranslationVersion(tb testing.TB, name string) io.Reader {
	tb.Helper()

	f, err := testdataFiles.Open(path.Join(prefix, name))
	if !assert.NoError(tb, err, "Must not error when trying to open file") {
		return bytes.NewBuffer(nil)
	}
	tb.Cleanup(func() {
		assert.NoError(tb, f.Close(), "Must not have issues trying to close static file")
	})
	return f
}

func NewExampleLogs(tb testing.TB, at Version) plog.Logs {
	tb.Helper()

	schemaURL := fmt.Sprint("https://example.com/", at.String())

	logs := plog.NewLogs()

	for i := 0; i < 10; i++ {
		log := logs.ResourceLogs().AppendEmpty()
		log.SetSchemaUrl(schemaURL)

		sl := log.ScopeLogs().AppendEmpty()
		sl.SetSchemaUrl(schemaURL)
		switch at {
		case Version{1, 7, 0}:
			log.Resource().Attributes().InsertString("test.name", tb.Name())

			l := sl.LogRecords().AppendEmpty()
			l.Attributes().InsertString("application.stacktrace", "func main() { panic('boom') }")
			l.SetSeverityText("ERROR")
			l.Body().SetStringVal("bad program")
		case Version{1, 5, 0}, Version{1, 4, 0}:
			// No changes to log during this versions
			fallthrough
		case Version{1, 2, 0}:
			log.Resource().Attributes().InsertString("test.name", tb.Name())

			l := sl.LogRecords().AppendEmpty()
			l.Attributes().InsertString("process.stacktrace", "func main() { panic('boom') }")
			l.SetSeverityText("ERROR")
			l.Body().SetStringVal("bad program")
		case Version{1, 1, 0}:
			log.Resource().Attributes().InsertString("test.suite", tb.Name())

			l := sl.LogRecords().AppendEmpty()
			l.Attributes().InsertString("process.stacktrace", "func main() { panic('boom') }")
			l.SetSeverityText("ERROR")
			l.Body().SetStringVal("bad program")
		case Version{1, 0, 0}:
			log.Resource().Attributes().InsertString("test-suite", tb.Name())

			l := sl.LogRecords().AppendEmpty()
			l.Attributes().InsertString("go.stacktrace", "func main() { panic('boom') }")
			l.SetSeverityText("ERROR")
			l.Body().SetStringVal("bad program")
		default:
			tb.Log("Unknown log version provided", at.String())
			tb.FailNow()
		}
	}

	return logs
}

func NewExampleMetrics(tb testing.TB, at Version) pmetric.Metrics {
	tb.Helper()

	schemaURL := fmt.Sprint("https://example.com/", at.String())

	metrics := pmetric.NewMetrics()
	for i := 0; i < 10; i++ {
		metric := metrics.ResourceMetrics().AppendEmpty()
		metric.SetSchemaUrl(schemaURL)

		sMetric := metric.ScopeMetrics().AppendEmpty()
		sMetric.SetSchemaUrl(schemaURL)

		for j := 0; j < 5; j++ {
			switch at {
			case Version{1, 7, 0}, Version{1, 5, 0}:
				metric.Resource().Attributes().InsertString("test.name", tb.Name())

				m := sMetric.Metrics().AppendEmpty()
				m.SetName("container.restart.total")
				m.SetDataType(pmetric.MetricDataTypeHistogram)
				hist := m.Histogram().DataPoints().AppendEmpty()
				hist.Attributes().InsertInt("container.exit.status", 124)

				m = sMetric.Metrics().AppendEmpty()
				m.SetName("container.restart.total")
				m.SetDataType(pmetric.MetricDataTypeExponentialHistogram)
				ehist := m.ExponentialHistogram().DataPoints().AppendEmpty()
				ehist.Attributes().InsertInt("container.exit.status", 124)

				m = sMetric.Metrics().AppendEmpty()
				m.SetName("container.restart.total")
				m.SetDataType(pmetric.MetricDataTypeSum)
				sum := m.Sum().DataPoints().AppendEmpty()
				sum.Attributes().InsertInt("container.exit.status", 124)

				m = sMetric.Metrics().AppendEmpty()
				m.SetName("container.restart.total")
				m.SetDataType(pmetric.MetricDataTypeSummary)
				summary := m.Summary().DataPoints().AppendEmpty()
				summary.Attributes().InsertInt("container.exit.status", 124)

				m = sMetric.Metrics().AppendEmpty()
				m.SetName("container.restart.total")
				m.SetDataType(pmetric.MetricDataTypeGauge)
				gauge := m.Gauge().DataPoints().AppendEmpty()
				gauge.Attributes().InsertInt("container.exit.status", 124)

			case Version{1, 4, 0}, Version{1, 2, 0}:
				metric.Resource().Attributes().InsertString("test.name", tb.Name())

				m := sMetric.Metrics().AppendEmpty()
				m.SetName("container.restart")
				m.SetDataType(pmetric.MetricDataTypeHistogram)
				hist := m.Histogram().DataPoints().AppendEmpty()
				hist.Attributes().InsertInt("container.exit.status", 124)

				m = sMetric.Metrics().AppendEmpty()
				m.SetName("container.restart")
				m.SetDataType(pmetric.MetricDataTypeExponentialHistogram)
				ehist := m.ExponentialHistogram().DataPoints().AppendEmpty()
				ehist.Attributes().InsertInt("container.exit.status", 124)

				m = sMetric.Metrics().AppendEmpty()
				m.SetName("container.restart")
				m.SetDataType(pmetric.MetricDataTypeSum)
				sum := m.Sum().DataPoints().AppendEmpty()
				sum.Attributes().InsertInt("container.exit.status", 124)

				m = sMetric.Metrics().AppendEmpty()
				m.SetName("container.restart")
				m.SetDataType(pmetric.MetricDataTypeSummary)
				summary := m.Summary().DataPoints().AppendEmpty()
				summary.Attributes().InsertInt("container.exit.status", 124)

				m = sMetric.Metrics().AppendEmpty()
				m.SetName("container.restart")
				m.SetDataType(pmetric.MetricDataTypeGauge)
				gauge := m.Gauge().DataPoints().AppendEmpty()
				gauge.Attributes().InsertInt("container.exit.status", 124)
			case Version{1, 1, 0}:
				metric.Resource().Attributes().InsertString("test.suite", tb.Name())

				m := sMetric.Metrics().AppendEmpty()
				m.SetName("container.restart")
				m.SetDataType(pmetric.MetricDataTypeHistogram)
				hist := m.Histogram().DataPoints().AppendEmpty()
				hist.Attributes().InsertInt("container.exit.status", 124)

				m = sMetric.Metrics().AppendEmpty()
				m.SetName("container.restart")
				m.SetDataType(pmetric.MetricDataTypeExponentialHistogram)
				ehist := m.ExponentialHistogram().DataPoints().AppendEmpty()
				ehist.Attributes().InsertInt("container.exit.status", 124)

				m = sMetric.Metrics().AppendEmpty()
				m.SetName("container.restart")
				m.SetDataType(pmetric.MetricDataTypeSum)
				sum := m.Sum().DataPoints().AppendEmpty()
				sum.Attributes().InsertInt("container.exit.status", 124)

				m = sMetric.Metrics().AppendEmpty()
				m.SetName("container.restart")
				m.SetDataType(pmetric.MetricDataTypeSummary)
				summary := m.Summary().DataPoints().AppendEmpty()
				summary.Attributes().InsertInt("container.exit.status", 124)

				m = sMetric.Metrics().AppendEmpty()
				m.SetName("container.restart")
				m.SetDataType(pmetric.MetricDataTypeGauge)
				gauge := m.Gauge().DataPoints().AppendEmpty()
				gauge.Attributes().InsertInt("container.exit.status", 124)
			case Version{1, 0, 0}:
				metric.Resource().Attributes().InsertString("test-suite", tb.Name())

				m := sMetric.Metrics().AppendEmpty()
				m.SetName("container.respawn")
				m.SetDataType(pmetric.MetricDataTypeHistogram)
				hist := m.Histogram().DataPoints().AppendEmpty()
				hist.Attributes().InsertInt("container-exit-code", 124)

				m = sMetric.Metrics().AppendEmpty()
				m.SetName("container.respawn")
				m.SetDataType(pmetric.MetricDataTypeExponentialHistogram)
				ehist := m.ExponentialHistogram().DataPoints().AppendEmpty()
				ehist.Attributes().InsertInt("container-exit-code", 124)

				m = sMetric.Metrics().AppendEmpty()
				m.SetName("container.respawn")
				m.SetDataType(pmetric.MetricDataTypeSum)
				sum := m.Sum().DataPoints().AppendEmpty()
				sum.Attributes().InsertInt("container-exit-code", 124)

				m = sMetric.Metrics().AppendEmpty()
				m.SetName("container.respawn")
				m.SetDataType(pmetric.MetricDataTypeSummary)
				summary := m.Summary().DataPoints().AppendEmpty()
				summary.Attributes().InsertInt("container-exit-code", 124)

				m = sMetric.Metrics().AppendEmpty()
				m.SetName("container.respawn")
				m.SetDataType(pmetric.MetricDataTypeGauge)
				gauge := m.Gauge().DataPoints().AppendEmpty()
				gauge.Attributes().InsertInt("container-exit-code", 124)
			default:
				tb.Log("Unknown metric version provided", at.String())
				tb.FailNow()
			}

		}
	}
	return metrics
}

func NewExampleSpans(tb testing.TB, at Version) ptrace.Traces {
	tb.Helper()

	schemaURL := fmt.Sprint("https://example.com/", at.String())
	traces := ptrace.NewTraces()

	for i := 0; i < 10; i++ {
		traces := traces.ResourceSpans().AppendEmpty()
		traces.SetSchemaUrl(schemaURL)

		spans := traces.ScopeSpans().AppendEmpty()
		spans.SetSchemaUrl(schemaURL)
		switch at {
		case Version{1, 7, 0}, Version{1, 5, 0}, Version{1, 4, 0}:
			traces.Resource().Attributes().InsertString("test.name", tb.Name())

			span := spans.Spans().AppendEmpty()
			span.SetName("POST /user/login")
			span.Attributes().InsertString("privacy.user.operation", "password encryption")

			span = spans.Spans().AppendEmpty()
			span.SetName("HTTP GET")
			span.Attributes().InsertString("privacy.user.operation", "password encryption")
			span.Attributes().InsertString("operation.failed.reason", "password too short")

			event := span.Events().AppendEmpty()
			event.SetName("stack_trace")
			event.Attributes().InsertString("privacy.net.user.ip", "127.0.0.1")

			event = span.Events().AppendEmpty()
			event.SetName("proxy.dial")
			event.Attributes().InsertString("proxy.addr", "127.0.0.1:5000")
			event.Attributes().InsertString("privacy.net.user.ip", "127.0.0.1")
		case Version{1, 2, 0}:
			traces.Resource().Attributes().InsertString("test.name", tb.Name())

			span := spans.Spans().AppendEmpty()
			span.SetName("POST /user/login")
			span.Attributes().InsertString("user.operation", "password encryption")

			span = spans.Spans().AppendEmpty()
			span.SetName("HTTP GET")
			span.Attributes().InsertString("user.operation", "password encryption")
			span.Attributes().InsertString("operation.failed.reason", "password too short")

			event := span.Events().AppendEmpty()
			event.SetName("stack_trace")
			event.Attributes().InsertString("net.user.ip", "127.0.0.1")

			event = span.Events().AppendEmpty()
			event.SetName("proxy.dial")
			event.Attributes().InsertString("proxy.addr", "127.0.0.1:5000")
			event.Attributes().InsertString("net.user.ip", "127.0.0.1")
		case Version{1, 1, 0}:
			traces.Resource().Attributes().InsertString("test.suite", tb.Name())

			span := spans.Spans().AppendEmpty()
			span.SetName("POST /user/login")
			span.Attributes().InsertString("user.operation", "password encryption")

			span = spans.Spans().AppendEmpty()
			span.SetName("HTTP GET")
			span.Attributes().InsertString("user.operation", "password encryption")
			span.Attributes().InsertString("operation.failed.reason", "password too short")

			event := span.Events().AppendEmpty()
			event.SetName("stack_trace")
			event.Attributes().InsertString("net.user.ip", "127.0.0.1")

			event = span.Events().AppendEmpty()
			event.SetName("proxy.dial")
			event.Attributes().InsertString("proxy.addr", "127.0.0.1:5000")
			event.Attributes().InsertString("net.user.ip", "127.0.0.1")
		case Version{1, 0, 0}:
			traces.Resource().Attributes().InsertString("test-suite", tb.Name())

			span := spans.Spans().AppendEmpty()
			span.SetName("POST /user/login")
			span.Attributes().InsertString("operation", "password encryption")

			span = spans.Spans().AppendEmpty()
			span.SetName("HTTP GET")
			span.Attributes().InsertString("operation", "password encryption")
			span.Attributes().InsertString("operation.failure", "password too short")

			event := span.Events().AppendEmpty()
			event.SetName("stacktrace")
			event.Attributes().InsertString("net.peer.ip", "127.0.0.1")

			event = span.Events().AppendEmpty()
			event.SetName("proxy.dial")
			event.Attributes().InsertString("proxy-addr", "127.0.0.1:5000")
			event.Attributes().InsertString("net.peer.ip", "127.0.0.1")
		default:
			tb.Log("Unknown trace version provided", at.String())
			tb.FailNow()
		}

	}

	return traces
}
