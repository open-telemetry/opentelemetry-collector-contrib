// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package translation

import (
	"embed"
	"fmt"
	"io"
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

func LoadTranslationVersion(tb testing.TB, name string) string {
	tb.Helper()

	f, err := testdataFiles.Open(path.Join(prefix, name))
	if !assert.NoError(tb, err, "Must not error when trying to open file") {
		return ""
	}
	tb.Cleanup(func() {
		assert.NoError(tb, f.Close(), "Must not have issues trying to close static file")
	})
	data, err := io.ReadAll(f)
	if !assert.NoError(tb, err, "Must not error when trying to read file") {
		return ""
	}
	return string(data)
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
			log.Resource().Attributes().PutStr("test.name", tb.Name())

			l := sl.LogRecords().AppendEmpty()
			l.Attributes().PutStr("application.stacktrace", "func main() { panic('boom') }")
			l.SetSeverityText("ERROR")
			l.Body().SetStr("bad program")
		case Version{1, 5, 0}, Version{1, 4, 0}:
			// No changes to log during this versions
			fallthrough
		case Version{1, 2, 0}:
			log.Resource().Attributes().PutStr("test.name", tb.Name())

			l := sl.LogRecords().AppendEmpty()
			l.Attributes().PutStr("process.stacktrace", "func main() { panic('boom') }")
			l.SetSeverityText("ERROR")
			l.Body().SetStr("bad program")
		case Version{1, 1, 0}:
			log.Resource().Attributes().PutStr("test.suite", tb.Name())

			l := sl.LogRecords().AppendEmpty()
			l.Attributes().PutStr("process.stacktrace", "func main() { panic('boom') }")
			l.SetSeverityText("ERROR")
			l.Body().SetStr("bad program")
		case Version{1, 0, 0}:
			log.Resource().Attributes().PutStr("test-suite", tb.Name())

			l := sl.LogRecords().AppendEmpty()
			l.Attributes().PutStr("go.stacktrace", "func main() { panic('boom') }")
			l.SetSeverityText("ERROR")
			l.Body().SetStr("bad program")
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
				metric.Resource().Attributes().PutStr("test.name", tb.Name())

				m := sMetric.Metrics().AppendEmpty()
				m.SetName("container.restart.total")
				m.SetEmptyHistogram()
				hist := m.Histogram().DataPoints().AppendEmpty()
				hist.Attributes().PutInt("container.exit.status", 124)

				m = sMetric.Metrics().AppendEmpty()
				m.SetName("container.restart.total")
				m.SetEmptyExponentialHistogram()
				ehist := m.ExponentialHistogram().DataPoints().AppendEmpty()
				ehist.Attributes().PutInt("container.exit.status", 124)

				m = sMetric.Metrics().AppendEmpty()
				m.SetName("container.restart.total")
				m.SetEmptySum()
				sum := m.Sum().DataPoints().AppendEmpty()
				sum.Attributes().PutInt("container.exit.status", 124)

				m = sMetric.Metrics().AppendEmpty()
				m.SetName("container.restart.total")
				m.SetEmptySummary()
				summary := m.Summary().DataPoints().AppendEmpty()
				summary.Attributes().PutInt("container.exit.status", 124)

				m = sMetric.Metrics().AppendEmpty()
				m.SetName("container.restart.total")
				m.SetEmptyGauge()
				gauge := m.Gauge().DataPoints().AppendEmpty()
				gauge.Attributes().PutInt("container.exit.status", 124)

			case Version{1, 4, 0}, Version{1, 2, 0}:
				metric.Resource().Attributes().PutStr("test.name", tb.Name())

				m := sMetric.Metrics().AppendEmpty()
				m.SetName("container.restart")
				m.SetEmptyHistogram()
				hist := m.Histogram().DataPoints().AppendEmpty()
				hist.Attributes().PutInt("container.exit.status", 124)

				m = sMetric.Metrics().AppendEmpty()
				m.SetName("container.restart")
				m.SetEmptyExponentialHistogram()
				ehist := m.ExponentialHistogram().DataPoints().AppendEmpty()
				ehist.Attributes().PutInt("container.exit.status", 124)

				m = sMetric.Metrics().AppendEmpty()
				m.SetName("container.restart")
				m.SetEmptySum()
				sum := m.Sum().DataPoints().AppendEmpty()
				sum.Attributes().PutInt("container.exit.status", 124)

				m = sMetric.Metrics().AppendEmpty()
				m.SetName("container.restart")
				m.SetEmptySummary()
				summary := m.Summary().DataPoints().AppendEmpty()
				summary.Attributes().PutInt("container.exit.status", 124)

				m = sMetric.Metrics().AppendEmpty()
				m.SetName("container.restart")
				m.SetEmptyGauge()
				gauge := m.Gauge().DataPoints().AppendEmpty()
				gauge.Attributes().PutInt("container.exit.status", 124)
			case Version{1, 1, 0}:
				metric.Resource().Attributes().PutStr("test.suite", tb.Name())

				m := sMetric.Metrics().AppendEmpty()
				m.SetName("container.restart")
				m.SetEmptyHistogram()
				hist := m.Histogram().DataPoints().AppendEmpty()
				hist.Attributes().PutInt("container.exit.status", 124)

				m = sMetric.Metrics().AppendEmpty()
				m.SetName("container.restart")
				m.SetEmptyExponentialHistogram()
				ehist := m.ExponentialHistogram().DataPoints().AppendEmpty()
				ehist.Attributes().PutInt("container.exit.status", 124)

				m = sMetric.Metrics().AppendEmpty()
				m.SetName("container.restart")
				m.SetEmptySum()
				sum := m.Sum().DataPoints().AppendEmpty()
				sum.Attributes().PutInt("container.exit.status", 124)

				m = sMetric.Metrics().AppendEmpty()
				m.SetName("container.restart")
				m.SetEmptySummary()
				summary := m.Summary().DataPoints().AppendEmpty()
				summary.Attributes().PutInt("container.exit.status", 124)

				m = sMetric.Metrics().AppendEmpty()
				m.SetName("container.restart")
				m.SetEmptyGauge()
				gauge := m.Gauge().DataPoints().AppendEmpty()
				gauge.Attributes().PutInt("container.exit.status", 124)
			case Version{1, 0, 0}:
				metric.Resource().Attributes().PutStr("test-suite", tb.Name())

				m := sMetric.Metrics().AppendEmpty()
				m.SetName("container.respawn")
				m.SetEmptyHistogram()
				hist := m.Histogram().DataPoints().AppendEmpty()
				hist.Attributes().PutInt("container-exit-code", 124)

				m = sMetric.Metrics().AppendEmpty()
				m.SetName("container.respawn")
				m.SetEmptyExponentialHistogram()
				ehist := m.ExponentialHistogram().DataPoints().AppendEmpty()
				ehist.Attributes().PutInt("container-exit-code", 124)

				m = sMetric.Metrics().AppendEmpty()
				m.SetName("container.respawn")
				m.SetEmptySum()
				sum := m.Sum().DataPoints().AppendEmpty()
				sum.Attributes().PutInt("container-exit-code", 124)

				m = sMetric.Metrics().AppendEmpty()
				m.SetName("container.respawn")
				m.SetEmptySummary()
				summary := m.Summary().DataPoints().AppendEmpty()
				summary.Attributes().PutInt("container-exit-code", 124)

				m = sMetric.Metrics().AppendEmpty()
				m.SetName("container.respawn")
				m.SetEmptyGauge()
				gauge := m.Gauge().DataPoints().AppendEmpty()
				gauge.Attributes().PutInt("container-exit-code", 124)
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
			traces.Resource().Attributes().PutStr("test.name", tb.Name())

			span := spans.Spans().AppendEmpty()
			span.SetName("POST /user/login")
			span.Attributes().PutStr("privacy.user.operation", "password encryption")

			span = spans.Spans().AppendEmpty()
			span.SetName("HTTP GET")
			span.Attributes().PutStr("privacy.user.operation", "password encryption")
			span.Attributes().PutStr("operation.failed.reason", "password too short")

			span = spans.Spans().AppendEmpty()
			span.SetName("HTTP GET1")
			span.Attributes().PutStr("privacy.user.operation", "password encryption")
			span.Attributes().PutStr("operation.failure", "password too short")

			event := span.Events().AppendEmpty()
			event.SetName("stack_trace")
			event.Attributes().PutStr("privacy.net.user.ip", "127.0.0.1")

			event = span.Events().AppendEmpty()
			event.SetName("proxy.dial")
			event.Attributes().PutStr("proxy.addr", "127.0.0.1:5000")
			event.Attributes().PutStr("privacy.net.user.ip", "127.0.0.1")
		case Version{1, 2, 0}:
			traces.Resource().Attributes().PutStr("test.name", tb.Name())

			span := spans.Spans().AppendEmpty()
			span.SetName("POST /user/login")
			span.Attributes().PutStr("user.operation", "password encryption")

			span = spans.Spans().AppendEmpty()
			span.SetName("HTTP GET")
			span.Attributes().PutStr("user.operation", "password encryption")
			span.Attributes().PutStr("operation.failed.reason", "password too short")

			span = spans.Spans().AppendEmpty()
			span.SetName("HTTP GET1")
			span.Attributes().PutStr("user.operation", "password encryption")
			span.Attributes().PutStr("operation.failure", "password too short")

			event := span.Events().AppendEmpty()
			event.SetName("stack_trace")
			event.Attributes().PutStr("net.user.ip", "127.0.0.1")

			event = span.Events().AppendEmpty()
			event.SetName("proxy.dial")
			event.Attributes().PutStr("proxy.addr", "127.0.0.1:5000")
			event.Attributes().PutStr("net.user.ip", "127.0.0.1")
		case Version{1, 1, 0}:
			traces.Resource().Attributes().PutStr("test.suite", tb.Name())

			span := spans.Spans().AppendEmpty()
			span.SetName("POST /user/login")
			span.Attributes().PutStr("user.operation", "password encryption")

			span = spans.Spans().AppendEmpty()
			span.SetName("HTTP GET")
			span.Attributes().PutStr("user.operation", "password encryption")
			span.Attributes().PutStr("operation.failed.reason", "password too short")

			span = spans.Spans().AppendEmpty()
			span.SetName("HTTP GET1")
			span.Attributes().PutStr("user.operation", "password encryption")
			span.Attributes().PutStr("operation.failure", "password too short")

			event := span.Events().AppendEmpty()
			event.SetName("stack_trace")
			event.Attributes().PutStr("net.user.ip", "127.0.0.1")

			event = span.Events().AppendEmpty()
			event.SetName("proxy.dial")
			event.Attributes().PutStr("proxy.addr", "127.0.0.1:5000")
			event.Attributes().PutStr("net.user.ip", "127.0.0.1")
		case Version{1, 0, 0}:
			traces.Resource().Attributes().PutStr("test-suite", tb.Name())

			span := spans.Spans().AppendEmpty()
			span.SetName("POST /user/login")
			span.Attributes().PutStr("operation", "password encryption")

			span = spans.Spans().AppendEmpty()
			span.SetName("HTTP GET")
			span.Attributes().PutStr("operation", "password encryption")
			span.Attributes().PutStr("operation.failure", "password too short")

			span = spans.Spans().AppendEmpty()
			span.SetName("HTTP GET1")
			span.Attributes().PutStr("operation", "password encryption")
			span.Attributes().PutStr("operation.failure", "password too short")

			event := span.Events().AppendEmpty()
			event.SetName("stacktrace")
			event.Attributes().PutStr("net.peer.ip", "127.0.0.1")

			event = span.Events().AppendEmpty()
			event.SetName("proxy.dial")
			event.Attributes().PutStr("proxy-addr", "127.0.0.1:5000")
			event.Attributes().PutStr("net.peer.ip", "127.0.0.1")
		default:
			tb.Log("Unknown trace version provided", at.String())
			tb.FailNow()
		}
	}

	return traces
}
