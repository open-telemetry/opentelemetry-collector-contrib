// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Code generated from semantic convention specification. DO NOT EDIT.
//
// Regenerate with `make generate-semconv-types`.

package otelsemconv // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/genainormalizerprocessor/internal/otelsemconv"

// typesV1_40_0 holds the gen_ai.* attributes defined at
// v1.40.0 of open-telemetry/semantic-conventions,
// excluding deprecated ones.
var typesV1_40_0 = map[string]kind{
	"gen_ai.agent.description":                 kindString,
	"gen_ai.agent.id":                          kindString,
	"gen_ai.agent.name":                        kindString,
	"gen_ai.agent.version":                     kindString,
	"gen_ai.conversation.id":                   kindString,
	"gen_ai.data_source.id":                    kindString,
	"gen_ai.embeddings.dimension.count":        kindInt,
	"gen_ai.evaluation.explanation":            kindString,
	"gen_ai.evaluation.name":                   kindString,
	"gen_ai.evaluation.score.label":            kindString,
	"gen_ai.evaluation.score.value":            kindDouble,
	"gen_ai.input.messages":                    kindAny,
	"gen_ai.operation.name":                    kindString,
	"gen_ai.output.messages":                   kindAny,
	"gen_ai.output.type":                       kindString,
	"gen_ai.prompt.name":                       kindString,
	"gen_ai.provider.name":                     kindString,
	"gen_ai.request.choice.count":              kindInt,
	"gen_ai.request.encoding_formats":          kindStringSlice,
	"gen_ai.request.frequency_penalty":         kindDouble,
	"gen_ai.request.max_tokens":                kindInt,
	"gen_ai.request.model":                     kindString,
	"gen_ai.request.presence_penalty":          kindDouble,
	"gen_ai.request.seed":                      kindInt,
	"gen_ai.request.stop_sequences":            kindStringSlice,
	"gen_ai.request.temperature":               kindDouble,
	"gen_ai.request.top_k":                     kindDouble,
	"gen_ai.request.top_p":                     kindDouble,
	"gen_ai.response.finish_reasons":           kindStringSlice,
	"gen_ai.response.id":                       kindString,
	"gen_ai.response.model":                    kindString,
	"gen_ai.retrieval.documents":               kindAny,
	"gen_ai.retrieval.query.text":              kindString,
	"gen_ai.system_instructions":               kindAny,
	"gen_ai.token.type":                        kindString,
	"gen_ai.tool.call.arguments":               kindAny,
	"gen_ai.tool.call.id":                      kindString,
	"gen_ai.tool.call.result":                  kindAny,
	"gen_ai.tool.definitions":                  kindAny,
	"gen_ai.tool.description":                  kindString,
	"gen_ai.tool.name":                         kindString,
	"gen_ai.tool.type":                         kindString,
	"gen_ai.usage.cache_creation.input_tokens": kindInt,
	"gen_ai.usage.cache_read.input_tokens":     kindInt,
	"gen_ai.usage.input_tokens":                kindInt,
	"gen_ai.usage.output_tokens":               kindInt,
}
