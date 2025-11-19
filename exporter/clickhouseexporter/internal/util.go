// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/clickhouseexporter/internal"

import (
	"slices"
	"strings"

	"github.com/ClickHouse/clickhouse-go/v2/lib/column"
	"github.com/ClickHouse/clickhouse-go/v2/lib/column/orderedmap"
	"go.opentelemetry.io/collector/pdata/pcommon"
	conventions "go.opentelemetry.io/otel/semconv/v1.27.0"
)

func GetServiceName(resAttr pcommon.Map) string {
	if v, ok := resAttr.Get(string(conventions.ServiceNameKey)); ok {
		return v.AsString()
	}

	return ""
}

func AttributesToMap(attributes pcommon.Map) column.IterableOrderedMap {
	return orderedmap.CollectN(func(yield func(string, string) bool) {
		allAttr := attributes.All()
		for k, v := range allAttr {
			yield(k, v.AsString())
		}
	}, attributes.Len())
}

// UniqueFlattenedAttributes converts a pcommon.Map into a slice of attributes. Paths are flattened and sorted.
func UniqueFlattenedAttributes(m pcommon.Map) []string {
	mLen := m.Len()
	if mLen == 0 {
		return nil
	}

	pathsSet := make(map[string]struct{}, mLen)
	paths := make([]string, 0, mLen)

	uniqueFlattenedAttributesNested("", &pathsSet, &paths, m)
	slices.Sort(paths)

	return paths
}

func uniqueFlattenedAttributesNested(pathPrefix string, pathsSet *map[string]struct{}, paths *[]string, m pcommon.Map) {
	m.Range(func(path string, v pcommon.Value) bool {
		if pathPrefix != "" {
			var b strings.Builder
			b.WriteString(pathPrefix)
			b.WriteRune('.')
			b.WriteString(path)
			path = b.String()
		}

		if v.Type() == pcommon.ValueTypeMap {
			uniqueFlattenedAttributesNested(path, pathsSet, paths, v.Map())
		} else if _, ok := (*pathsSet)[path]; !ok {
			(*pathsSet)[path] = struct{}{}
			*paths = append(*paths, path)
		}

		return true
	})
}
