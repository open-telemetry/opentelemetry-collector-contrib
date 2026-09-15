// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package crewai holds the attribute mapping table used to normalize
// CrewAI-instrumented spans to the OTel GenAI semantic conventions. CrewAI's
// built-in telemetry emits unprefixed attribute keys (no "crewai." namespace),
// so source-side keys are raw string literals observed in real telemetry.
//
// Reference: https://github.com/crewAIInc/crewAI
package crewai // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/genainormalizerprocessor/internal/crewai"

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/genainormalizerprocessor/internal/otelsemconv"
)

// LookupTable maps CrewAI attribute keys to the OTel GenAI target keys. All
// targets are typed strings, so no value transformation is required; the
// processor copies values verbatim after type coercion.
var LookupTable = map[string]string{
	// Agent identity
	"agent_role": otelsemconv.GenAIAgentName,

	// Conversation / crew tracking
	"crew_id": otelsemconv.GenAIConversationID,
}
