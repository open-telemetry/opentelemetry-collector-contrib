// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureblobexporter

import (
	"fmt"
	"strings"
	"testing"

	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pipeline"
	"go.uber.org/zap"
)

func benchLogs(resources, recordsPerResource int, distinctNames bool) plog.Logs {
	logs := plog.NewLogs()
	body := strings.Repeat("x", 100)
	for i := range resources {
		rl := logs.ResourceLogs().AppendEmpty()
		name := "same-activity"
		if distinctNames {
			name = fmt.Sprintf("activity-%d", i)
		}
		rl.Resource().Attributes().PutStr("activity-id", name)
		sl := rl.ScopeLogs().AppendEmpty()
		for range recordsPerResource {
			sl.LogRecords().AppendEmpty().Body().SetStr(body)
		}
	}
	return logs
}

func benchExporter(b *testing.B, templateEnabled bool) *azureBlobExporter {
	b.Helper()
	c := newPartitionTestConfig(`{{ getResourceLogAttr . 0 "activity-id" }}.json`, templateEnabled)
	e := newAzureBlobExporter(c, zap.NewNop(), pipeline.SignalLogs)
	if err := e.start(b.Context(), nil); err != nil {
		b.Fatal(err)
	}
	return e
}

func BenchmarkPartitionLogsByBlobName(b *testing.B) {
	for _, bc := range []struct {
		name            string
		templateEnabled bool
		distinct        bool
	}{
		{"template_disabled", false, false},
		{"all_same_name", true, false},
		{"all_distinct_names", true, true},
	} {
		b.Run(bc.name, func(b *testing.B) {
			e := benchExporter(b, bc.templateEnabled)
			logs := benchLogs(10, 100, bc.distinct)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				groups := e.partitionLogsByBlobName(logs)
				_ = groups
			}
		})
	}
}

func BenchmarkMarshalLogsBaseline(b *testing.B) {
	e := benchExporter(b, true)
	logs := benchLogs(10, 100, false)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if _, err := e.marshaller.marshalLogs(logs); err != nil {
			b.Fatal(err)
		}
	}
}
