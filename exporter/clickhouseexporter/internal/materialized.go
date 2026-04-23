// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/clickhouseexporter/internal"

import (
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.uber.org/zap"
)

// MaterializedColumnDef defines a client-side column extraction at runtime.
type MaterializedColumnDef struct {
	Map    string
	Key    string
	Column string
}

// ValidateMaterializedColumns filters materialized column definitions to only those
// whose target Column exists in the actual table schema. Logs a warning for any
// configured columns that are not found in the table.
func ValidateMaterializedColumns(defs []MaterializedColumnDef, tableColumns []string, logger *zap.Logger) []MaterializedColumnDef {
	if len(defs) == 0 {
		return nil
	}

	columnSet := make(map[string]bool, len(tableColumns))
	for _, col := range tableColumns {
		columnSet[col] = true
	}

	var validated []MaterializedColumnDef
	for _, def := range defs {
		if columnSet[def.Column] {
			validated = append(validated, def)
		} else {
			logger.Warn("materialized column not found in table schema, skipping",
				zap.String("column", def.Column),
				zap.String("map", def.Map),
				zap.String("key", def.Key),
			)
		}
	}

	return validated
}

// BuildMaterializedSQLFragments returns SQL fragments for injecting materialized columns
// into INSERT statements. Returns (columnNames, placeholders) where columnNames is like
// ", col1, col2" and placeholders is like ", ?, ?". Both are empty strings when defs is empty.
func BuildMaterializedSQLFragments(defs []MaterializedColumnDef) (string, string) {
	if len(defs) == 0 {
		return "", ""
	}

	var colNames strings.Builder
	var placeholders strings.Builder
	for _, def := range defs {
		colNames.WriteString(", ")
		colNames.WriteString(def.Column)
		placeholders.WriteString(", ?")
	}

	return colNames.String(), placeholders.String()
}

// ExtractMaterializedValues extracts attribute values for each materialized column definition.
// The sources map keys are attribute column names (e.g. "ResourceAttributes") mapped to
// their pcommon.Map values. Returns a string slice in the same order as defs.
// Missing keys or missing source maps default to empty string.
func ExtractMaterializedValues(defs []MaterializedColumnDef, sources map[string]pcommon.Map) []string {
	values := make([]string, len(defs))
	for i, def := range defs {
		if m, ok := sources[def.Map]; ok {
			if v, found := m.Get(def.Key); found {
				values[i] = v.AsString()
			}
		}
	}

	return values
}

// AppendMaterializedValues appends extracted materialized column values to the columnValues slice.
func AppendMaterializedValues(columnValues []any, defs []MaterializedColumnDef, sources map[string]pcommon.Map) []any {
	if len(defs) == 0 {
		return columnValues
	}

	values := ExtractMaterializedValues(defs, sources)
	for _, v := range values {
		columnValues = append(columnValues, v)
	}

	return columnValues
}
