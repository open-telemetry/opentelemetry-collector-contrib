// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package helper // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
)

// NewIdentifierConfig creates a new identifier config with default values
func NewIdentifierConfig() IdentifierConfig {
	return IdentifierConfig{
		Resource: make(map[string]ExprStringConfig),
	}
}

// IdentifierConfig is the configuration of a resource identifier
type IdentifierConfig struct {
	Resource map[string]ExprStringConfig `mapstructure:"resource"`
}

// Build will build an identifier from the supplied configuration
func (c IdentifierConfig) Build() (Identifier, error) {
	identifier := Identifier{
		resource: make(map[string]*ExprString),
	}

	for k, v := range c.Resource {
		exprString, err := v.Build()
		if err != nil {
			return identifier, err
		}

		identifier.resource[k] = exprString
	}

	return identifier, nil
}

// Identifier is a helper that adds values to the resource of an entry
type Identifier struct {
	resource map[string]*ExprString
}

// Identify will add values to the resource of an entry
func (i *Identifier) Identify(e *entry.Entry) error {
	if len(i.resource) == 0 {
		return nil
	}

	env := GetExprEnv(e)
	defer PutExprEnv(env)

	for k, v := range i.resource {
		rendered, err := v.Render(env)
		if err != nil {
			return err
		}
		e.AddResourceKey(k, rendered)
	}

	return nil
}
