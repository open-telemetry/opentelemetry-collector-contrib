// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package otelsemconv is the single source of truth for the OTel semantic-
// conventions version this processor targets. Bumping versions: update the
// import path below, verify referenced constants still exist, and bump any
// version references in public-facing docs.
package otelsemconv // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/genainormalizerprocessor/internal/otelsemconv"

import conventions "go.opentelemetry.io/otel/semconv/v1.40.0"

// SchemaURL is the OTel semconv schema URL for the targeted version.
const SchemaURL = conventions.SchemaURL

// GenAI attribute keys as plain strings for use as pdata keys.
var (
	GenAIOperationName     = string(conventions.GenAIOperationNameKey)
	GenAIUsageInputTokens  = string(conventions.GenAIUsageInputTokensKey)
	GenAIUsageOutputTokens = string(conventions.GenAIUsageOutputTokensKey)
	GenAIRequestModel      = string(conventions.GenAIRequestModelKey)
	GenAIProviderName      = string(conventions.GenAIProviderNameKey)
	GenAIInputMessages     = string(conventions.GenAIInputMessagesKey)
	GenAIOutputMessages    = string(conventions.GenAIOutputMessagesKey)
	GenAIToolName          = string(conventions.GenAIToolNameKey)
	GenAIToolDescription   = string(conventions.GenAIToolDescriptionKey)
	GenAIToolCallArguments = string(conventions.GenAIToolCallArgumentsKey)
	GenAIToolCallID        = string(conventions.GenAIToolCallIDKey)
	GenAIAgentName         = string(conventions.GenAIAgentNameKey)
	GenAIConversationID    = string(conventions.GenAIConversationIDKey)
)
