// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package db // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/redactionprocessor/internal/db"

import (
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	semconv128 "go.opentelemetry.io/otel/semconv/v1.28.0"
	semconv138 "go.opentelemetry.io/otel/semconv/v1.38.0"
)

// SanitizeSpanName obfuscates the span name if it represents a database statement.
// Returns the obfuscated name, whether a change was made, and any error encountered.
func SanitizeSpanName(span ptrace.Span, obfuscator *Obfuscator) (string, bool, error) {
	if obfuscator == nil || !obfuscator.HasObfuscators() {
		return "", false, nil
	}

	name := span.Name()

	kind := span.Kind()
	if kind != ptrace.SpanKindClient && kind != ptrace.SpanKindServer && kind != ptrace.SpanKindInternal {
		return "", false, nil
	}

	dbSystem := GetDBSystem(span.Attributes())
	if dbSystem == "" {
		return name, false, nil
	}

	obfuscated, err := obfuscator.ObfuscateWithSystem(name, dbSystem)
	if err != nil {
		return "", false, err
	}
	if obfuscated == name {
		return "", false, nil
	}
	return obfuscated, true, nil
}

func GetDBSystem(attributes pcommon.Map) string {
	if system := getStringAttrLower(attributes, string(semconv138.DBSystemNameKey)); system != "" {
		return system
	}
	return getStringAttrLower(attributes, string(semconv128.DBSystemKey))
}

func getStringAttrLower(attributes pcommon.Map, key string) string {
	if value, ok := attributes.Get(key); ok && value.Type() == pcommon.ValueTypeStr {
		return strings.ToLower(value.Str())
	}
	return ""
}
