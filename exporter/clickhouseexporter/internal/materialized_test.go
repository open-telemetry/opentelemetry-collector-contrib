// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

func TestValidateMaterializedColumns(t *testing.T) {
	logger := zaptest.NewLogger(t)

	tests := []struct {
		name         string
		defs         []MaterializedColumnDef
		tableColumns []string
		wantLen      int
	}{
		{
			name:    "nil defs",
			defs:    nil,
			wantLen: 0,
		},
		{
			name: "all columns present",
			defs: []MaterializedColumnDef{
				{Map: "ResourceAttributes", Key: "service.name", Column: "mat_svc"},
				{Map: "SpanAttributes", Key: "http.method", Column: "mat_method"},
			},
			tableColumns: []string{"Timestamp", "mat_svc", "mat_method", "ResourceAttributes"},
			wantLen:      2,
		},
		{
			name: "some columns missing",
			defs: []MaterializedColumnDef{
				{Map: "ResourceAttributes", Key: "service.name", Column: "mat_svc"},
				{Map: "SpanAttributes", Key: "http.method", Column: "mat_method"},
			},
			tableColumns: []string{"Timestamp", "mat_svc", "ResourceAttributes"},
			wantLen:      1,
		},
		{
			name: "no columns present",
			defs: []MaterializedColumnDef{
				{Map: "ResourceAttributes", Key: "service.name", Column: "mat_svc"},
			},
			tableColumns: []string{"Timestamp", "ResourceAttributes"},
			wantLen:      0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateMaterializedColumns(tt.defs, tt.tableColumns, logger)
			assert.Len(t, result, tt.wantLen)
		})
	}
}

func TestBuildMaterializedSQLFragments(t *testing.T) {
	tests := []struct {
		name             string
		defs             []MaterializedColumnDef
		wantColNames     string
		wantPlaceholders string
	}{
		{
			name:             "empty defs",
			defs:             nil,
			wantColNames:     "",
			wantPlaceholders: "",
		},
		{
			name: "single def",
			defs: []MaterializedColumnDef{
				{Map: "ResourceAttributes", Key: "service.name", Column: "mat_svc"},
			},
			wantColNames:     ", mat_svc",
			wantPlaceholders: ", ?",
		},
		{
			name: "multiple defs",
			defs: []MaterializedColumnDef{
				{Map: "ResourceAttributes", Key: "service.name", Column: "mat_svc"},
				{Map: "SpanAttributes", Key: "http.method", Column: "mat_method"},
			},
			wantColNames:     ", mat_svc, mat_method",
			wantPlaceholders: ", ?, ?",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			colNames, placeholders := BuildMaterializedSQLFragments(tt.defs)
			assert.Equal(t, tt.wantColNames, colNames)
			assert.Equal(t, tt.wantPlaceholders, placeholders)
		})
	}
}

func TestExtractMaterializedValues(t *testing.T) {
	resAttr := pcommon.NewMap()
	resAttr.PutStr("service.name", "my-service")
	resAttr.PutStr("host.name", "host1")

	spanAttr := pcommon.NewMap()
	spanAttr.PutStr("http.method", "GET")

	sources := map[string]pcommon.Map{
		"ResourceAttributes": resAttr,
		"SpanAttributes":     spanAttr,
	}

	tests := []struct {
		name string
		defs []MaterializedColumnDef
		want []string
	}{
		{
			name: "key present",
			defs: []MaterializedColumnDef{
				{Map: "ResourceAttributes", Key: "service.name", Column: "mat_svc"},
			},
			want: []string{"my-service"},
		},
		{
			name: "key missing defaults to empty string",
			defs: []MaterializedColumnDef{
				{Map: "ResourceAttributes", Key: "missing.key", Column: "mat_missing"},
			},
			want: []string{""},
		},
		{
			name: "source map missing defaults to empty string",
			defs: []MaterializedColumnDef{
				{Map: "LogAttributes", Key: "some.key", Column: "mat_log"},
			},
			want: []string{""},
		},
		{
			name: "multiple extractions",
			defs: []MaterializedColumnDef{
				{Map: "ResourceAttributes", Key: "service.name", Column: "mat_svc"},
				{Map: "SpanAttributes", Key: "http.method", Column: "mat_method"},
				{Map: "ResourceAttributes", Key: "missing.key", Column: "mat_missing"},
			},
			want: []string{"my-service", "GET", ""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractMaterializedValues(tt.defs, sources)
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestAppendMaterializedValues(t *testing.T) {
	resAttr := pcommon.NewMap()
	resAttr.PutStr("service.name", "my-service")

	sources := map[string]pcommon.Map{
		"ResourceAttributes": resAttr,
	}

	t.Run("empty defs returns unchanged slice", func(t *testing.T) {
		columnValues := []any{"a", "b"}
		result := AppendMaterializedValues(columnValues, nil, sources)
		assert.Equal(t, []any{"a", "b"}, result)
	})

	t.Run("appends extracted values", func(t *testing.T) {
		defs := []MaterializedColumnDef{
			{Map: "ResourceAttributes", Key: "service.name", Column: "mat_svc"},
		}
		columnValues := []any{"a", "b"}
		result := AppendMaterializedValues(columnValues, defs, sources)
		assert.Equal(t, []any{"a", "b", "my-service"}, result)
	})
}

func TestValidateMaterializedColumnsLogsWarning(t *testing.T) {
	logger := zap.NewNop()

	defs := []MaterializedColumnDef{
		{Map: "ResourceAttributes", Key: "service.name", Column: "missing_col"},
	}
	tableColumns := []string{"Timestamp", "ResourceAttributes"}

	result := ValidateMaterializedColumns(defs, tableColumns, logger)
	assert.Empty(t, result)
}
