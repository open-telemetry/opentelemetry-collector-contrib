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

func benchLogs(resources, recordsPerResource, destinations int) plog.Logs {
	logs := plog.NewLogs()
	body := strings.Repeat("x", 100)
	for i := range resources {
		rl := logs.ResourceLogs().AppendEmpty()
		name := fmt.Sprintf("activity-%04d", i%destinations)
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
		resources       int
		destinations    int
	}{
		{"template_disabled", false, 10, 10},
		{"all_same_name", true, 10, 1},
		{"all_distinct_names", true, 10, 10},
		// Same resource shape as the next case; only destination cardinality differs.
		{"resources=1000/destinations=1", true, 1000, 1},
		{"resources=1000/destinations=1000", true, 1000, 1000},
	} {
		b.Run(bc.name, func(b *testing.B) {
			e := benchExporter(b, bc.templateEnabled)
			// Total record volume is fixed at 1000 across all cases.
			logs := benchLogs(bc.resources, 1000/bc.resources, bc.destinations)
			wantGroups := bc.destinations
			if !bc.templateEnabled {
				wantGroups = 1
			}
			if got := len(e.partitionLogsByBlobName(logs)); got != wantGroups {
				b.Fatalf("expected %d groups, got %d", wantGroups, got)
			}
			b.ReportAllocs()
			for b.Loop() {
				e.partitionLogsByBlobName(logs)
			}
		})
	}
}

func BenchmarkMarshalLogsBaseline(b *testing.B) {
	for _, resources := range []int{10, 1000} {
		b.Run(fmt.Sprintf("resources=%d", resources), func(b *testing.B) {
			e := benchExporter(b, true)
			logs := benchLogs(resources, 1000/resources, 1)
			b.ReportAllocs()
			for b.Loop() {
				if _, err := e.marshaller.marshalLogs(logs); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
