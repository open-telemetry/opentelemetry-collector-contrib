// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package migrate provides the functionality to take action
// on specific schema fields that are consumed by the translation package.
package migrate // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor/internal/migrate"

type StateSelector int

const (
	_ StateSelector = iota
	StateSelectorApply
	StateSelectorRollback
)
