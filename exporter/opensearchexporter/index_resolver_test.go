// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opensearchexporter

import (
	"testing"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
)

func TestIndexResolver_ResolveIndex(t *testing.T) {
	resolver := newIndexResolver("ss4o_logs", "default", "namespace")
	ts := time.Date(2025, 6, 7, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name          string
		indexPattern  string
		fallback      string
		timeFormat    string
		resourceAttrs map[string]string
		scopeAttrs    map[string]string
		itemAttrs     map[string]string
		expected      string
	}{
		{
			name:          "service name",
			indexPattern:  "otel-%{service.name}",
			fallback:      "default-service",
			timeFormat:    "yyyy.MM.dd",
			resourceAttrs: map[string]string{"service.name": "myservice"},
			expected:      "otel-myservice-2025.06.07",
		},
		{
			name:          "missing service name",
			indexPattern:  "otel-%{service.name}",
			fallback:      "default-service",
			timeFormat:    "yyyy.MM.dd",
			resourceAttrs: map[string]string{},
			expected:      "otel-default-service-2025.06.07",
		},
		{
			name:          "no time format",
			indexPattern:  "otel-%{service.name}",
			fallback:      "default-service",
			timeFormat:    "",
			resourceAttrs: map[string]string{"service.name": "myservice"},
			expected:      "otel-myservice",
		},
		{
			name:          "empty index",
			indexPattern:  "",
			fallback:      "",
			timeFormat:    "yyyy.MM.dd",
			resourceAttrs: map[string]string{"service.name": "myservice"},
			expected:      "ss4o_logs-default-namespace-2025.06.07",
		},
		{
			name:          "custom attribute",
			indexPattern:  "otel-%{custom.label}",
			fallback:      "fallback",
			timeFormat:    "yyyy.MM.dd",
			resourceAttrs: map[string]string{"custom.label": "myapp"},
			expected:      "otel-myapp-2025.06.07",
		},
		{
			name:          "unknown placeholder",
			indexPattern:  "otel-%{nonexistent}",
			fallback:      "",
			timeFormat:    "yyyy.MM.dd",
			resourceAttrs: map[string]string{},
			expected:      "otel-unknown-2025.06.07",
		},
		{
			name:          "multiple placeholders",
			indexPattern:  "%{service.name}-%{env}-logs",
			fallback:      "default",
			timeFormat:    "yyyy.MM.dd",
			resourceAttrs: map[string]string{"service.name": "myservice", "env": "prod"},
			expected:      "myservice-prod-logs-2025.06.07",
		},
		{
			name:          "multiple placeholders with fallback",
			indexPattern:  "%{service.name}-%{missing}-logs",
			fallback:      "fallback",
			timeFormat:    "yyyy.MM.dd",
			resourceAttrs: map[string]string{"service.name": "myservice"},
			expected:      "myservice-fallback-logs-2025.06.07",
		},
		{
			name:          "attribute precedence item over scope",
			indexPattern:  "test-%{key}",
			fallback:      "",
			timeFormat:    "",
			resourceAttrs: map[string]string{"key": "resource"},
			scopeAttrs:    map[string]string{"key": "scope"},
			itemAttrs:     map[string]string{"key": "item"},
			expected:      "test-item",
		},
		{
			name:          "attribute precedence scope over resource",
			indexPattern:  "test-%{key}",
			fallback:      "",
			timeFormat:    "",
			resourceAttrs: map[string]string{"key": "resource"},
			scopeAttrs:    map[string]string{"key": "scope"},
			itemAttrs:     map[string]string{},
			expected:      "test-scope",
		},
		{
			name:          "attribute precedence resource",
			indexPattern:  "test-%{key}",
			fallback:      "",
			timeFormat:    "",
			resourceAttrs: map[string]string{"key": "resource"},
			scopeAttrs:    map[string]string{},
			itemAttrs:     map[string]string{},
			expected:      "test-resource",
		},
		{
			name:          "scope name",
			indexPattern:  "test-%{scope.name}",
			fallback:      "",
			timeFormat:    "",
			resourceAttrs: map[string]string{},
			scopeAttrs:    map[string]string{"scope.name": "my.scope"},
			itemAttrs:     map[string]string{},
			expected:      "test-my.scope",
		},
		{
			name:          "scope version",
			indexPattern:  "test-%{scope.version}",
			fallback:      "",
			timeFormat:    "",
			resourceAttrs: map[string]string{},
			scopeAttrs:    map[string]string{"scope.version": "1.0.0"},
			itemAttrs:     map[string]string{},
			expected:      "test-1.0.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			keys := resolver.extractPlaceholderKeys(tt.indexPattern)
			itemMap := pcommon.NewMap()
			for k, v := range tt.itemAttrs {
				itemMap.PutStr(k, v)
			}
			timeSuffix := resolver.calculateTimeSuffix(tt.timeFormat, ts)
			index := resolver.resolveIndexName(tt.indexPattern, tt.fallback, itemMap, keys, tt.scopeAttrs, tt.resourceAttrs, timeSuffix)
			if index != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, index)
			}
		})
	}
}
