// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package custom

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func TestTransform_MappedStringValue(t *testing.T) {
	rules := map[string]map[string]string{
		"gen_ai.operation.name": {
			"chat_completion": "chat",
		},
	}
	transform := Transform(rules)

	src := pcommon.NewValueStr("chat_completion")
	dst := pcommon.NewValueEmpty()
	transform("gen_ai.operation.name", src, dst)

	require.Equal(t, pcommon.ValueTypeStr, dst.Type())
	assert.Equal(t, "chat", dst.Str())
}

func TestTransform_CaseInsensitiveSourceValue(t *testing.T) {
	rules := map[string]map[string]string{
		"gen_ai.operation.name": {"chat_completion": "chat"},
	}
	transform := Transform(rules)

	src := pcommon.NewValueStr("Chat_Completion")
	dst := pcommon.NewValueEmpty()
	transform("gen_ai.operation.name", src, dst)

	assert.Equal(t, "chat", dst.Str())
}

func TestTransform_UnmatchedSourceValuePassesThrough(t *testing.T) {
	rules := map[string]map[string]string{
		"gen_ai.operation.name": {"chat_completion": "chat"},
	}
	transform := Transform(rules)

	src := pcommon.NewValueStr("not_in_rules")
	dst := pcommon.NewValueEmpty()
	transform("gen_ai.operation.name", src, dst)

	assert.Equal(t, "not_in_rules", dst.Str())
}

func TestTransform_TargetWithNoRulesPassesThrough(t *testing.T) {
	rules := map[string]map[string]string{
		"gen_ai.operation.name": {"chat_completion": "chat"},
	}
	transform := Transform(rules)

	src := pcommon.NewValueStr("anything")
	dst := pcommon.NewValueEmpty()
	transform("gen_ai.request.model", src, dst)

	assert.Equal(t, "anything", dst.Str())
}

func TestTransform_NonStringSourcePassesThrough(t *testing.T) {
	rules := map[string]map[string]string{
		"gen_ai.usage.input_tokens": {"100": "999"},
	}
	transform := Transform(rules)

	src := pcommon.NewValueInt(100)
	dst := pcommon.NewValueEmpty()
	transform("gen_ai.usage.input_tokens", src, dst)

	require.Equal(t, pcommon.ValueTypeInt, dst.Type())
	assert.Equal(t, int64(100), dst.Int())
}

func TestTransform_NilRulesPassesThrough(t *testing.T) {
	transform := Transform(nil)

	src := pcommon.NewValueStr("anything")
	dst := pcommon.NewValueEmpty()
	transform("gen_ai.operation.name", src, dst)

	assert.Equal(t, "anything", dst.Str())
}

func TestTransform_EmptyRuleMapForKeyPassesThrough(t *testing.T) {
	rules := map[string]map[string]string{
		"gen_ai.operation.name": {},
	}
	transform := Transform(rules)

	src := pcommon.NewValueStr("anything")
	dst := pcommon.NewValueEmpty()
	transform("gen_ai.operation.name", src, dst)

	assert.Equal(t, "anything", dst.Str())
}
