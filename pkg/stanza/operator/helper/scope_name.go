// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package helper // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/errors"
)

// ScopeNameParser is a helper that parses severity onto an entry.
type ScopeNameParser struct {
	ParseFrom entry.Field `mapstructure:"parse_from,omitempty"`
}

// NewScopeNameParser creates a new scope parser with default values
func NewScopeNameParser() ScopeNameParser {
	return ScopeNameParser{}
}

// Parse will parse severity from a field and attach it to the entry
func (p *ScopeNameParser) Parse(ent *entry.Entry) error {
	value, ok := ent.Get(p.ParseFrom)
	if !ok {
		return errors.NewError(
			"log entry does not have the expected parse_from field",
			"ensure that all entries forwarded to this parser contain the parse_from field",
			"parse_from", p.ParseFrom.String(),
		)
	}

	strVal, ok := value.(string)
	if !ok {
		err := ent.Set(p.ParseFrom, value)
		if err != nil {
			return errors.Wrap(err, "parse_from field does not contain a string")
		}
		return errors.NewError(
			"parse_from field does not contain a string",
			"ensure that all entries forwarded to this parser contain a string in the parse_from field",
			"parse_from", p.ParseFrom.String(),
		)
	}

	ent.ScopeName = strVal
	return nil
}
