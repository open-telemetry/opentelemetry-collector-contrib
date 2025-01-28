// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package common // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/common"

import (
	"fmt"
	"strings"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

var _ ottl.StatementsGetter = (*ContextStatements)(nil)

type ContextID string

const (
	Resource  ContextID = "resource"
	Scope     ContextID = "scope"
	Span      ContextID = "span"
	SpanEvent ContextID = "spanevent"
	Metric    ContextID = "metric"
	DataPoint ContextID = "datapoint"
	Log       ContextID = "log"
)

func (c *ContextID) UnmarshalText(text []byte) error {
	str := ContextID(strings.ToLower(string(text)))
	switch str {
	case Resource, Scope, Span, SpanEvent, Metric, DataPoint, Log:
		*c = str
		return nil
	default:
		return fmt.Errorf("unknown context %v", str)
	}
}

type ContextStatements struct {
	Context    ContextID `mapstructure:"context"`
	Conditions []string  `mapstructure:"conditions"`
	Statements []string  `mapstructure:"statements"`
	// ErrorMode determines how the processor reacts to errors that occur while processing
	// this group of statements. When provided, it overrides the default Config ErrorMode.
	ErrorMode ottl.ErrorMode `mapstructure:"error_mode"`
	// SharedCache is experimental and subject to change or removal in the future.
	// Although it's configurable via `mapstructure`, users won't be able to set it on their
	// configurations, as it's currently meant for internal use only, and it's validated by
	// the transformprocessor Config unmarshaller function.
	SharedCache bool `mapstructure:"shared_cache"`
}

func (c ContextStatements) GetStatements() []string {
	return c.Statements
}

func toContextStatements(statements any) (*ContextStatements, error) {
	contextStatements, ok := statements.(ContextStatements)
	if !ok {
		return nil, fmt.Errorf("invalid context statements type, expected: common.ContextStatements, got: %T", statements)
	}
	return &contextStatements, nil
}
