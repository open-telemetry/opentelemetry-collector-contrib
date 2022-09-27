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

// NewAttributerConfig creates a new attributer config with default values
func NewAttributerConfig() AttributerConfig {
	return AttributerConfig{
		Attributes: make(map[string]ExprStringConfig),
	}
}

// AttributerConfig is the configuration of a attributer
type AttributerConfig struct {
	Attributes map[string]ExprStringConfig `mapstructure:"attributes"`
}

// Build will build a attributer from the supplied configuration
func (c AttributerConfig) Build() (Attributer, error) {
	attributer := Attributer{
		attributes: make(map[string]*ExprString),
	}

	for k, v := range c.Attributes {
		exprString, err := v.Build()
		if err != nil {
			return attributer, err
		}

		attributer.attributes[k] = exprString
	}

	return attributer, nil
}

// Attributer is a helper that adds attributes to an entry
type Attributer struct {
	attributes map[string]*ExprString
}

// Attribute will add attributes to an entry
func (l *Attributer) Attribute(e *entry.Entry) error {
	if len(l.attributes) == 0 {
		return nil
	}

	env := GetExprEnv(e)
	defer PutExprEnv(env)

	for k, v := range l.attributes {
		rendered, err := v.Render(env)
		if err != nil {
			return err
		}
		e.AddAttribute(k, rendered)
	}

	return nil
}
