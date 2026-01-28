// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package db

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap/zaptest"
)

func TestSanitizeSpanNameRequiresDBSystem(t *testing.T) {
	obfuscator := NewObfuscator(DBSanitizerConfig{
		SQLConfig: SQLConfig{Enabled: true},
	}, zaptest.NewLogger(t))

	span := ptrace.NewSpan()
	span.SetKind(ptrace.SpanKindClient)
	span.SetName("SELECT 1")

	_, ok, err := SanitizeSpanName(span, obfuscator)
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestSanitizeSpanNameWithDBSystemName(t *testing.T) {
	obfuscator := NewObfuscator(DBSanitizerConfig{
		SQLConfig: SQLConfig{Enabled: true},
	}, zaptest.NewLogger(t))

	span := ptrace.NewSpan()
	span.SetKind(ptrace.SpanKindClient)
	span.SetName("SELECT * FROM users WHERE id = 42")
	span.Attributes().PutStr("db.system.name", "mysql")
	span.Attributes().PutStr("db.statement", "SELECT * FROM users WHERE id = 42")

	result, ok, err := SanitizeSpanName(span, obfuscator)
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, "SELECT * FROM users WHERE id = ?", result)
}

func TestSanitizeSpanNameNilObfuscator(t *testing.T) {
	span := ptrace.NewSpan()
	span.SetKind(ptrace.SpanKindClient)
	span.SetName("SELECT * FROM users")

	result, ok, err := SanitizeSpanName(span, nil)
	require.NoError(t, err)
	assert.False(t, ok)
	assert.Empty(t, result)
}

func TestSanitizeSpanNameUnsupportedSpanKinds(t *testing.T) {
	obfuscator := NewObfuscator(DBSanitizerConfig{
		SQLConfig: SQLConfig{Enabled: true},
	}, zaptest.NewLogger(t))

	testCases := []struct {
		name string
		kind ptrace.SpanKind
	}{
		{"producer", ptrace.SpanKindProducer},
		{"consumer", ptrace.SpanKindConsumer},
		{"unspecified", ptrace.SpanKindUnspecified},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			span := ptrace.NewSpan()
			span.SetKind(tc.kind)
			span.SetName("SELECT * FROM users WHERE id = 42")
			span.Attributes().PutStr("db.system", "mysql")

			result, ok, err := SanitizeSpanName(span, obfuscator)
			require.NoError(t, err)
			assert.False(t, ok)
			assert.Empty(t, result)
		})
	}
}

func TestSanitizeSpanNameUnchanged(t *testing.T) {
	obfuscator := NewObfuscator(DBSanitizerConfig{
		SQLConfig: SQLConfig{Enabled: true},
	}, zaptest.NewLogger(t))

	span := ptrace.NewSpan()
	span.SetKind(ptrace.SpanKindClient)
	// Use a span name that won't be changed by SQL obfuscation
	span.SetName("SHOW TABLES")
	span.Attributes().PutStr("db.system", "mysql")

	result, ok, err := SanitizeSpanName(span, obfuscator)
	require.NoError(t, err)
	assert.False(t, ok)
	assert.Empty(t, result)
}

func TestSanitizeSpanNameNoObfuscators(t *testing.T) {
	obfuscator := NewObfuscator(DBSanitizerConfig{}, zaptest.NewLogger(t))

	span := ptrace.NewSpan()
	span.SetKind(ptrace.SpanKindClient)
	span.SetName("SELECT * FROM users WHERE id = 42")
	span.Attributes().PutStr("db.system", "mysql")

	result, ok, err := SanitizeSpanName(span, obfuscator)
	require.NoError(t, err)
	assert.False(t, ok)
	assert.Empty(t, result)
}

func TestSanitizeSpanNameServerKind(t *testing.T) {
	obfuscator := NewObfuscator(DBSanitizerConfig{
		SQLConfig: SQLConfig{Enabled: true},
	}, zaptest.NewLogger(t))

	span := ptrace.NewSpan()
	span.SetKind(ptrace.SpanKindServer)
	span.SetName("SELECT * FROM users WHERE id = 42")
	span.Attributes().PutStr("db.system", "mysql")

	result, ok, err := SanitizeSpanName(span, obfuscator)
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, "SELECT * FROM users WHERE id = ?", result)
}

func TestSanitizeSpanNameInternalKind(t *testing.T) {
	obfuscator := NewObfuscator(DBSanitizerConfig{
		SQLConfig: SQLConfig{Enabled: true},
	}, zaptest.NewLogger(t))

	span := ptrace.NewSpan()
	span.SetKind(ptrace.SpanKindInternal)
	span.SetName("SELECT * FROM users WHERE id = 42")
	span.Attributes().PutStr("db.system", "mysql")

	result, ok, err := SanitizeSpanName(span, obfuscator)
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, "SELECT * FROM users WHERE id = ?", result)
}

func TestGetDBSystemLegacyAttribute(t *testing.T) {
	m := pcommon.NewMap()
	m.PutStr("db.system", "PostgreSQL")
	assert.Equal(t, "postgresql", GetDBSystem(m))
}

func TestGetDBSystemNewAttribute(t *testing.T) {
	m := pcommon.NewMap()
	m.PutStr("db.system.name", "MySQL")
	assert.Equal(t, "mysql", GetDBSystem(m))
}

func TestGetDBSystemPreferNewAttribute(t *testing.T) {
	m := pcommon.NewMap()
	m.PutStr("db.system", "postgresql")
	m.PutStr("db.system.name", "mysql")
	// db.system.name takes precedence
	assert.Equal(t, "mysql", GetDBSystem(m))
}

func TestGetDBSystemNoAttribute(t *testing.T) {
	m := pcommon.NewMap()
	assert.Empty(t, GetDBSystem(m))
}

func TestGetDBSystemNonStringValue(t *testing.T) {
	m := pcommon.NewMap()
	m.PutInt("db.system", 123)
	assert.Empty(t, GetDBSystem(m))
}
