// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azuremonitorexporter

import (
	"strings"
	"testing"

	"github.com/microsoft/ApplicationInsights-Go/appinsights/contracts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

func TestSanitizeKeepingGenAIContent(t *testing.T) {
	long := strings.Repeat("a", 20000)
	data := contracts.NewRemoteDependencyData()
	data.Properties = map[string]string{}
	data.Properties["gen_ai.input.messages"] = long
	data.Properties["other"] = long

	warnings := sanitizeKeepingGenAIContent(data.Properties, data.Sanitize)

	assert.Equal(t, long, data.Properties["gen_ai.input.messages"])
	assert.Len(t, data.Properties["other"], 8192)
	require.Len(t, warnings, 1)
	assert.NotContains(t, warnings[0], "gen_ai.input.messages")
}

func TestSanitizeKeepingGenAIContentTruncatesAtLargerLimit(t *testing.T) {
	data := contracts.NewMessageData()
	data.Properties = map[string]string{}
	data.Properties["gen_ai.system_instructions"] = strings.Repeat("a", genAIContentPropertyMaxLength+10)

	warnings := sanitizeKeepingGenAIContent(data.Properties, data.Sanitize)

	assert.Len(t, data.Properties["gen_ai.system_instructions"], genAIContentPropertyMaxLength)
	require.Len(t, warnings, 1)
	assert.Contains(t, warnings[0], "gen_ai.system_instructions")
}

func TestTruncateUTF8(t *testing.T) {
	// "é" is two bytes, so cutting at 3 bytes would split the second one.
	assert.Equal(t, "aé", truncateUTF8("aéé", 3))
	assert.Equal(t, "aéé", truncateUTF8("aéé", 5))
	assert.Equal(t, "abc", truncateUTF8("abc", 10))
}

func TestSpanToEnvelopesKeepsGenAIContent(t *testing.T) {
	long := strings.Repeat("m", 20000)
	span := getSpan("chat", ptrace.SpanKindClient, map[string]any{
		"gen_ai.input.messages": long,
		"other":                 long,
	})

	envelopes, err := spanToEnvelopes(defaultResource, defaultInstrumentationLibrary, span, true, httpStatusCodeSuccessConfig{}, nil, zap.NewNop())
	require.NoError(t, err)

	data := envelopes[0].Data.(*contracts.Data).BaseData.(*contracts.RemoteDependencyData)
	assert.Equal(t, long, data.Properties["gen_ai.input.messages"])
	assert.Len(t, data.Properties["other"], 8192)
}

func TestLogRecordToEnvelopeKeepsGenAIContent(t *testing.T) {
	long := strings.Repeat("m", 20000)
	resource, scope, logRecord := getTestLogRecord(1)
	logRecord.Attributes().PutStr("gen_ai.output.messages", long)

	envelope := getLogPacker().LogRecordToEnvelope(logRecord, resource, scope)

	data := envelope.Data.(*contracts.Data).BaseData.(*contracts.MessageData)
	assert.Equal(t, long, data.Properties["gen_ai.output.messages"])
}
