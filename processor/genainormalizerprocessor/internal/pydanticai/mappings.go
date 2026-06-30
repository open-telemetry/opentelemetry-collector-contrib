// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package pydanticai holds the attribute mapping table used to normalize
// PydanticAI-instrumented spans to the OTel GenAI semantic conventions.
// PydanticAI's instrumentation emits unprefixed attribute keys (no
// "pydantic_ai." namespace), so source-side keys are raw string literals
// observed in real telemetry.
//
// Reference: https://github.com/pydantic/pydantic-ai
package pydanticai // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/genainormalizerprocessor/internal/pydanticai"

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/genainormalizerprocessor/internal/otelsemconv"
)

// LookupTable maps PydanticAI attribute keys to the OTel GenAI target keys.
// The target is a typed string, so no value transformation is required; the
// processor copies the value verbatim after type coercion.
var LookupTable = map[string]string{
	// Agent identity
	"agent_name": otelsemconv.GenAIAgentName,
}
