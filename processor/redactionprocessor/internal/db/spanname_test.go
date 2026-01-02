// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package db

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
