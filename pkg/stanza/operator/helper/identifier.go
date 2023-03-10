// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
