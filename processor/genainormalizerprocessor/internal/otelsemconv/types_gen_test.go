// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otelsemconv

import (
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	conventions "go.opentelemetry.io/otel/semconv/v1.40.0"
)

// TestDefaultTargetMatchesSchemaURL checks that the conventions import (via
// SchemaURL), defaultTarget, and the generated table all name the same release.
func TestDefaultTargetMatchesSchemaURL(t *testing.T) {
	assert.Truef(t, strings.HasSuffix(SchemaURL, "/"+strings.TrimPrefix(defaultTarget, "v")),
		"SchemaURL %q does not name defaultTarget %q", SchemaURL, defaultTarget)

	_, ok := typesByTarget[defaultTarget]
	assert.Truef(t, ok, "no generated table for defaultTarget %q", defaultTarget)
}

// TestTargetTypesCoversDeclaredKeys checks that every key this package exports
// is defined by the targeted registry. Coerce passes unknown keys through, so a
// key renamed upstream would otherwise go unenforced.
func TestTargetTypesCoversDeclaredKeys(t *testing.T) {
	declared := map[string]string{
		"GenAIAgentName":               GenAIAgentName,
		"GenAIConversationID":          GenAIConversationID,
		"GenAIInputMessages":           GenAIInputMessages,
		"GenAIOperationName":           GenAIOperationName,
		"GenAIOutputMessages":          GenAIOutputMessages,
		"GenAIProviderName":            GenAIProviderName,
		"GenAIRequestFrequencyPenalty": GenAIRequestFrequencyPenalty,
		"GenAIRequestMaxTokens":        GenAIRequestMaxTokens,
		"GenAIRequestModel":            GenAIRequestModel,
		"GenAIRequestPresencePenalty":  GenAIRequestPresencePenalty,
		"GenAIRequestStopSequences":    GenAIRequestStopSequences,
		"GenAIRequestTemperature":      GenAIRequestTemperature,
		"GenAIRequestTopK":             GenAIRequestTopK,
		"GenAIRequestTopP":             GenAIRequestTopP,
		"GenAIResponseFinishReasons":   GenAIResponseFinishReasons,
		"GenAIResponseModel":           GenAIResponseModel,
		"GenAIToolCallArguments":       GenAIToolCallArguments,
		"GenAIToolCallID":              GenAIToolCallID,
		"GenAIToolDefinitions":         GenAIToolDefinitions,
		"GenAIToolDescription":         GenAIToolDescription,
		"GenAIToolName":                GenAIToolName,
		"GenAIUsageInputTokens":        GenAIUsageInputTokens,
		"GenAIUsageOutputTokens":       GenAIUsageOutputTokens,
	}
	for name, key := range declared {
		_, ok := targetTypes[key]
		assert.Truef(t, ok, "%s (%q) is not defined by the %s registry", name, key, defaultTarget)
	}
}

// TestTargetTypesMatchesSemconvConstructors derives the expected type for every
// gen_ai.* attribute the semconv Go library exposes a typed constructor for, and
// compares it against the generated table. The derivation is the same one the
// package performed at init before the table was generated: read the
// constructor's argument type, and take the key from the KeyValue it returns.
//
// Nothing here is hand-written, so the check is independent of the table rather
// than a restatement of it.
func TestTargetTypesMatchesSemconvConstructors(t *testing.T) {
	for _, ctor := range []any{
		conventions.GenAIAgentName,
		conventions.GenAIConversationID,
		conventions.GenAIRequestFrequencyPenalty,
		conventions.GenAIRequestMaxTokens,
		conventions.GenAIRequestModel,
		conventions.GenAIRequestPresencePenalty,
		conventions.GenAIRequestStopSequences,
		conventions.GenAIRequestTemperature,
		conventions.GenAIRequestTopK,
		conventions.GenAIRequestTopP,
		conventions.GenAIResponseFinishReasons,
		conventions.GenAIResponseModel,
		conventions.GenAIToolCallID,
		conventions.GenAIToolDescription,
		conventions.GenAIToolName,
		conventions.GenAIUsageInputTokens,
		conventions.GenAIUsageOutputTokens,
	} {
		key, argType := describeConstructor(t, ctor)
		t.Run(key, func(t *testing.T) {
			got, ok := targetTypes[key]
			require.Truef(t, ok, "%s has a typed constructor but is not in the %s table", key, defaultTarget)
			assert.Equalf(t, kindOfGoType(t, argType), got, "%s", key)
		})
	}
}

// describeConstructor calls a semconv constructor with a zero value and reports
// the attribute key it produces along with the type it accepts.
func describeConstructor(t *testing.T, ctor any) (string, reflect.Type) {
	t.Helper()
	ft := reflect.TypeOf(ctor)
	require.Equal(t, reflect.Func, ft.Kind())
	require.Equal(t, 1, ft.NumIn())
	require.Equal(t, 1, ft.NumOut())

	in := ft.In(0)
	var out []reflect.Value
	if ft.IsVariadic() {
		// A variadic ...T parameter arrives as []T; CallSlice passes it whole.
		out = reflect.ValueOf(ctor).CallSlice([]reflect.Value{reflect.MakeSlice(in, 0, 0)})
	} else {
		out = reflect.ValueOf(ctor).Call([]reflect.Value{reflect.Zero(in)})
	}
	kv, ok := out[0].Interface().(attribute.KeyValue)
	require.True(t, ok)
	return string(kv.Key), in
}

// kindOfGoType maps a constructor argument type onto the kind the table should
// carry for it.
func kindOfGoType(t *testing.T, typ reflect.Type) kind {
	t.Helper()
	switch typ.Kind() {
	case reflect.Int, reflect.Int64:
		return kindInt
	case reflect.Float64:
		return kindDouble
	case reflect.String:
		return kindString
	case reflect.Bool:
		return kindBoolean
	case reflect.Slice:
		require.Equal(t, reflect.String, typ.Elem().Kind())
		return kindStringSlice
	}
	t.Fatalf("unhandled constructor argument type %s", typ)
	return kindAny
}

// TestTargetTypesForKeysWithoutConstructors covers the keys the semconv Go
// library exposes only as a *Key constant, so there is no constructor to derive
// a type from. These expectations are hand-written and read from the registry.
//
// gen_ai.operation.name and gen_ai.provider.name are the two keys whose
// enforcement this table introduces: both are closed string enums in the
// registry, and neither has a constructor, so they were previously unchecked.
func TestTargetTypesForKeysWithoutConstructors(t *testing.T) {
	for key, want := range map[string]kind{
		GenAIInputMessages:     kindAny,
		GenAIOutputMessages:    kindAny,
		GenAIToolCallArguments: kindAny,
		GenAIToolDefinitions:   kindAny,
		GenAIOperationName:     kindString,
		GenAIProviderName:      kindString,
	} {
		t.Run(key, func(t *testing.T) {
			assert.Equal(t, want, targetTypes[key])
		})
	}
}
