// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package langchain holds the attribute mapping table used to normalize
// LangChain / LangGraph-instrumented spans to the OTel GenAI semantic
// conventions. LangChain publishes no Go semantic-conventions package, so
// source-side keys are raw string literals observed in real telemetry.
//
// Reference: https://github.com/langchain-ai/langchain
package langchain // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/genainormalizerprocessor/internal/langchain"

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/genainormalizerprocessor/internal/otelsemconv"
)

// LookupTable maps LangChain / LangGraph attribute keys to the OTel GenAI
// target keys. All targets are typed strings, so no value transformation is
// required; the processor copies values verbatim after type coercion.
var LookupTable = map[string]string{
	// Conversation / thread tracking
	"lc.metadata.thread_id": otelsemconv.GenAIConversationID,

	// Agent identity (LangGraph node name)
	"langgraph.node.name": otelsemconv.GenAIAgentName,
}
