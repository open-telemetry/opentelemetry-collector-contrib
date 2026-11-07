// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package transformconfig // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/transformconfig"

import (
	"fmt"
	"strings"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

var _ ottl.StatementsGetter = (*ContextStatements)(nil)

// ContextID is the name of the OTTL context a group of statements runs against.
type ContextID string

const (
	Resource  ContextID = "resource"
	Scope     ContextID = "scope"
	Span      ContextID = "span"
	SpanEvent ContextID = "spanevent"
	Metric    ContextID = "metric"
	DataPoint ContextID = "datapoint"
	Exemplar  ContextID = "exemplar"
	Log       ContextID = "log"
	Profile   ContextID = "profile"
)

// UnmarshalText sets the ContextID from its case-insensitive name and rejects unknown contexts.
func (c *ContextID) UnmarshalText(text []byte) error {
	str := ContextID(strings.ToLower(string(text)))
	switch str {
	case Resource, Scope, Span, SpanEvent, Metric, DataPoint, Exemplar, Log, Profile:
		*c = str
		return nil
	default:
		return fmt.Errorf("unknown context %v", str)
	}
}

// ContextStatements is a group of OTTL statements and conditions executed together.
type ContextStatements struct {
	Context    ContextID `mapstructure:"context"`
	Conditions []string  `mapstructure:"conditions"`
	Statements []string  `mapstructure:"statements"`
	// ErrorMode determines how the processor reacts to errors that occur while processing
	// this group of statements. When provided, it overrides the default Config ErrorMode.
	ErrorMode ottl.ErrorMode `mapstructure:"error_mode"`
	// SharedCache enables sharing per-context caches between all statement groups
	// with this option enabled in the same processor instance.
	SharedCache bool `mapstructure:"shared_cache"`
}

// GetStatements returns the group's OTTL statements.
func (c ContextStatements) GetStatements() []string {
	return c.Statements
}
