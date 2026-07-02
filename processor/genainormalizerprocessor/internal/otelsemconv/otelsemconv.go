// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package otelsemconv pins the OTel semantic-conventions version this
// processor targets and exports the gen_ai.* attribute keys it emits, along
// with the Go type each key is defined to carry.
//
// Bumping versions: update the conventions import path below, verify any
// referenced symbols still exist, and bump version references in public docs.
//
// Adding a new attribute: prefer typed(conventions.GenAIFooBar) — the typed
// constructor's argument type drives Coerce, which is what enforces spec
// types at write time. For keys whose spec type is "any" (e.g.
// gen_ai.input.messages, gen_ai.operation.name) the semconv library exposes
// only a *Key constant; convert it with string(conventions.GenAIFooBarKey).
// Values for these keys pass through Coerce verbatim, with no type
// enforcement. If a typed constructor exists, use typed.
package otelsemconv // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/genainormalizerprocessor/internal/otelsemconv"

import (
	"reflect"

	"go.opentelemetry.io/otel/attribute"
	conventions "go.opentelemetry.io/otel/semconv/v1.40.0"
)

// SchemaURL is the OTel semconv schema URL for the targeted version.
const SchemaURL = conventions.SchemaURL

// targetTypes maps each gen_ai.* key registered via typed() to the Go type
// the corresponding semconv constructor accepts. Variadic constructors (e.g.
// ...string) yield a slice type. Keys whose spec type is "any" are not
// registered (their declarations use string(...) directly); Coerce treats
// absent keys as opaque and passes values through.
var targetTypes = map[string]reflect.Type{}

// typed registers a gen_ai.* attribute whose semconv definition is a typed
// constructor (e.g. func(string) attribute.KeyValue) and returns the
// attribute key as a string for use as a pdata map key. The constructor's
// argument type is recorded so Coerce can enforce it at write time.
//
// Both the key string and the type registration come from the same ctor
// symbol — there's no separate *Key constant to drift.
func typed(ctor any) string {
	t := reflect.TypeOf(ctor)
	if t.Kind() != reflect.Func || t.NumIn() != 1 || t.NumOut() != 1 {
		panic("otelsemconv: typed expects a 1-arg, 1-return semconv constructor")
	}
	in := t.In(0)
	var out []reflect.Value
	if t.IsVariadic() {
		// Variadic ...T: param type already arrives as []T. CallSlice passes
		// the slice as the variadic arg.
		out = reflect.ValueOf(ctor).CallSlice([]reflect.Value{reflect.MakeSlice(in, 0, 0)})
	} else {
		out = reflect.ValueOf(ctor).Call([]reflect.Value{reflect.Zero(in)})
	}
	kv := out[0].Interface().(attribute.KeyValue)
	name := string(kv.Key)
	targetTypes[name] = in
	return name
}

// enumValue extracts the string value from a semconv enum KeyValue (e.g.
// conventions.GenAIOperationNameChat). The semconv library exposes these as
// pre-built attribute.KeyValue with a String value, so we unwrap once at
// init rather than at every call site.
func enumValue(kv attribute.KeyValue) string {
	return kv.Value.AsString()
}

// GenAI attribute keys, grouped by subnamespace. Adding a new attribute is
// one line: typed(conventions.GenAIFoo) for keys with a typed constructor,
// or string(conventions.GenAIFooKey) for keys whose spec type is "any".
//
//nolint:gochecknoglobals // canonical key registry
var (
	// gen_ai.agent.*
	GenAIAgentName = typed(conventions.GenAIAgentName)

	// gen_ai.conversation.*
	GenAIConversationID = typed(conventions.GenAIConversationID)

	// gen_ai.input.*
	GenAIInputMessages = string(conventions.GenAIInputMessagesKey)

	// gen_ai.operation.*
	GenAIOperationName = string(conventions.GenAIOperationNameKey)

	// gen_ai.output.*
	GenAIOutputMessages = string(conventions.GenAIOutputMessagesKey)

	// gen_ai.provider.*
	GenAIProviderName = string(conventions.GenAIProviderNameKey)

	// gen_ai.request.*
	GenAIRequestFrequencyPenalty = typed(conventions.GenAIRequestFrequencyPenalty)
	GenAIRequestMaxTokens        = typed(conventions.GenAIRequestMaxTokens)
	GenAIRequestModel            = typed(conventions.GenAIRequestModel)
	GenAIRequestPresencePenalty  = typed(conventions.GenAIRequestPresencePenalty)
	GenAIRequestStopSequences    = typed(conventions.GenAIRequestStopSequences)
	GenAIRequestTemperature      = typed(conventions.GenAIRequestTemperature)
	GenAIRequestTopK             = typed(conventions.GenAIRequestTopK)
	GenAIRequestTopP             = typed(conventions.GenAIRequestTopP)

	// gen_ai.response.*
	GenAIResponseFinishReasons = typed(conventions.GenAIResponseFinishReasons)
	GenAIResponseModel         = typed(conventions.GenAIResponseModel)

	// gen_ai.tool.*
	GenAIToolCallArguments = string(conventions.GenAIToolCallArgumentsKey)
	GenAIToolCallID        = typed(conventions.GenAIToolCallID)
	GenAIToolDefinitions   = string(conventions.GenAIToolDefinitionsKey)
	GenAIToolDescription   = typed(conventions.GenAIToolDescription)
	GenAIToolName          = typed(conventions.GenAIToolName)

	// gen_ai.usage.*
	GenAIUsageInputTokens  = typed(conventions.GenAIUsageInputTokens)
	GenAIUsageOutputTokens = typed(conventions.GenAIUsageOutputTokens)
)

// gen_ai.operation.name enum values, sourced from the semconv library.
// Source-side normalizers (openinference, openllmetry) fold their own
// span-kind/operation-type enums onto these.
//
//nolint:gochecknoglobals // canonical enum value registry
var (
	GenAIOperationNameChat           = enumValue(conventions.GenAIOperationNameChat)
	GenAIOperationNameEmbeddings     = enumValue(conventions.GenAIOperationNameEmbeddings)
	GenAIOperationNameExecuteTool    = enumValue(conventions.GenAIOperationNameExecuteTool)
	GenAIOperationNameInvokeAgent    = enumValue(conventions.GenAIOperationNameInvokeAgent)
	GenAIOperationNameRetrieval      = enumValue(conventions.GenAIOperationNameRetrieval)
	GenAIOperationNameTextCompletion = enumValue(conventions.GenAIOperationNameTextCompletion)

	// GenAIOperationNameInvokeWorkflow is not part of the standard enum. It
	// is emitted by openllmetry's traceloop.span.kind=workflow path; we keep
	// it as a centralized constant so callers don't need to hard-code the
	// raw string.
	GenAIOperationNameInvokeWorkflow = "invoke_workflow"
)
