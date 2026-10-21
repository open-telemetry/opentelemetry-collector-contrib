// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package springai normalizes spans emitted by Spring AI's Micrometer
// observations to the OTel GenAI semantic conventions. Spring AI already
// writes most gen_ai.* keys itself, but the Micrometer tracing bridge records
// every value as a string, and tool and conversation details live under
// spring.ai.* keys.
//
// Reference: https://github.com/spring-projects/spring-ai/tree/cba03cdd219d9ee45d30c691349bfc733f6d1110/spring-ai-model/src/main/java/org/springframework/ai/tool/observation
package springai // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/genainormalizerprocessor/internal/springai"

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/genainormalizerprocessor/internal/otelsemconv"
)

// LookupTable maps Spring AI attribute keys to the OTel GenAI target keys.
var LookupTable = map[string]string{
	"spring.ai.tool.definition.name":        otelsemconv.GenAIToolName,
	"spring.ai.tool.definition.description": otelsemconv.GenAIToolDescription,
	"spring.ai.tool.type":                   otelsemconv.GenAIToolType,
	"spring.ai.tool.call.id":                otelsemconv.GenAIToolCallID,
	"spring.ai.tool.call.arguments":         otelsemconv.GenAIToolCallArguments,
	"spring.ai.tool.call.result":            otelsemconv.GenAIToolCallResult,

	// Only set when the application passes ChatMemory.CONVERSATION_ID.
	"spring.ai.chat.client.conversation.id": otelsemconv.GenAIConversationID,
}
