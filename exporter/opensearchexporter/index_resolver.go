// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opensearchexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/opensearchexporter"

import (
	"regexp"
	"strings"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
)

// indexResolver handles dynamic index name resolution for logs and traces
type indexResolver struct {
	placeholderPattern *regexp.Regexp
	defaultPrefix      string
	defaultDataset     string
	defaultNamespace   string
}

// newIndexResolver creates a new index resolver instance
func newIndexResolver(defaultPrefix, defaultDataset, defaultNamespace string) *indexResolver {
	return &indexResolver{
		placeholderPattern: regexp.MustCompile(`%\{([^}]+)\}`),
		defaultPrefix:      defaultPrefix,
		defaultDataset:     defaultDataset,
		defaultNamespace:   defaultNamespace,
	}
}

// getDefaultIndexName provides default index naming for backward compatibility
func (r *indexResolver) getDefaultIndexName() string {
	return strings.Join([]string{r.defaultPrefix, r.defaultDataset, r.defaultNamespace}, "-")
}

// extractPlaceholderKeys extracts unique placeholder keys from the index pattern
func (r *indexResolver) extractPlaceholderKeys(template string) []string {
	matches := r.placeholderPattern.FindAllStringSubmatch(template, -1)
	keySet := make(map[string]bool)
	for _, match := range matches {
		if len(match) > 1 {
			keySet[match[1]] = true
		}
	}
	keys := make([]string, 0, len(keySet))
	for key := range keySet {
		keys = append(keys, key)
	}
	return keys
}

// collectResourceAttributes collects resource attributes for the specified keys
func (*indexResolver) collectResourceAttributes(resource pcommon.Resource, keys []string) map[string]string {
	attrs := make(map[string]string)
	for _, key := range keys {
		if v, ok := resource.Attributes().Get(key); ok {
			attrs[key] = v.AsString()
		}
	}
	return attrs
}

// collectScopeAttributes collects scope attributes (including scope.name and scope.version) for the specified keys
func (*indexResolver) collectScopeAttributes(scope pcommon.InstrumentationScope, keys []string) map[string]string {
	attrs := make(map[string]string)
	for _, key := range keys {
		switch key {
		case "scope.name":
			if scope.Name() != "" {
				attrs[key] = scope.Name()
			}
		case "scope.version":
			if scope.Version() != "" {
				attrs[key] = scope.Version()
			}
		default:
			if v, ok := scope.Attributes().Get(key); ok {
				attrs[key] = v.AsString()
			}
		}
	}
	return attrs
}

// resolveIndexName handles the common logic for resolving index names with placeholders
func (r *indexResolver) resolveIndexName(indexPattern, fallback string, itemAttrs pcommon.Map, keys []string, scopeAttributes, resourceAttributes map[string]string, timeSuffix string) string {
	if indexPattern == "" {
		return r.getDefaultIndexName() + timeSuffix
	}
	itemAttributes := make(map[string]string)
	for _, key := range keys {
		if v, ok := itemAttrs.Get(key); ok {
			itemAttributes[key] = v.AsString()
		}
	}
	indexName := r.placeholderPattern.ReplaceAllStringFunc(indexPattern, func(match string) string {
		key := r.placeholderPattern.FindStringSubmatch(match)[1]
		if val, ok := itemAttributes[key]; ok && val != "" {
			return val
		}
		if val, ok := scopeAttributes[key]; ok && val != "" {
			return val
		}
		if val, ok := resourceAttributes[key]; ok && val != "" {
			return val
		}
		if fallback != "" {
			return fallback
		}
		return "unknown"
	})

	return indexName + timeSuffix
}

// calculateTimeSuffix calculates the time suffix string for the given format and timestamp
func (*indexResolver) calculateTimeSuffix(timeFormat string, timestamp time.Time) string {
	if timeFormat != "" {
		return "-" + timestamp.Format(convertGoTimeFormat(timeFormat))
	}
	return ""
}

// convertGoTimeFormat converts a Java-style date format to Go's time format
func convertGoTimeFormat(format string) string {
	// Support yyyy, yy, MM, dd, HH, mm, ss -> 2006, 06, 01, 02, 15, 04, 05
	f := format
	f = strings.ReplaceAll(f, "yyyy", "2006")
	f = strings.ReplaceAll(f, "yy", "06")
	f = strings.ReplaceAll(f, "MM", "01")
	f = strings.ReplaceAll(f, "dd", "02")
	f = strings.ReplaceAll(f, "HH", "15")
	f = strings.ReplaceAll(f, "mm", "04")
	f = strings.ReplaceAll(f, "ss", "05")
	return f
}
