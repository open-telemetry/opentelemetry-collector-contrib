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

package operator // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"

import (
	"encoding/json"
	"fmt"

	"go.opentelemetry.io/collector/confmap"
	"go.uber.org/zap"
)

// Config is the configuration of an operator
type Config struct {
	Builder
}

// NewConfig wraps the builder interface in a concrete struct
func NewConfig(b Builder) Config {
	return Config{Builder: b}
}

// Builder is an entity that can build a single operator
type Builder interface {
	ID() string
	Type() string
	Build(*zap.SugaredLogger) (Operator, error)
	SetID(string)
}

// UnmarshalJSON will unmarshal a config from JSON.
func (c *Config) UnmarshalJSON(bytes []byte) error {
	var typeUnmarshaller struct {
		Type string
	}

	if err := json.Unmarshal(bytes, &typeUnmarshaller); err != nil {
		return err
	}

	if typeUnmarshaller.Type == "" {
		return fmt.Errorf("missing required field 'type'")
	}

	builderFunc, ok := DefaultRegistry.Lookup(typeUnmarshaller.Type)
	if !ok {
		return fmt.Errorf("unsupported type '%s'", typeUnmarshaller.Type)
	}

	builder := builderFunc()
	if err := json.Unmarshal(bytes, builder); err != nil {
		return fmt.Errorf("unmarshal to %s: %w", typeUnmarshaller.Type, err)
	}

	c.Builder = builder
	return nil
}

// UnmarshalYAML will unmarshal a config from YAML.
func (c *Config) UnmarshalYAML(unmarshal func(interface{}) error) error {
	rawConfig := map[string]interface{}{}
	err := unmarshal(&rawConfig)
	if err != nil {
		return fmt.Errorf("failed to unmarshal yaml to base config: %w", err)
	}

	typeInterface, ok := rawConfig["type"]
	if !ok {
		return fmt.Errorf("missing required field 'type'")
	}

	typeString, ok := typeInterface.(string)
	if !ok {
		return fmt.Errorf("non-string type %T for field 'type'", typeInterface)
	}

	builderFunc, ok := DefaultRegistry.Lookup(typeString)
	if !ok {
		return fmt.Errorf("unsupported type '%s'", typeString)
	}

	builder := builderFunc()
	if err = unmarshal(builder); err != nil {
		return fmt.Errorf("unmarshal to %s: %w", typeString, err)
	}

	c.Builder = builder
	return nil
}

func (c *Config) Unmarshal(component *confmap.Conf) error {
	if !component.IsSet("type") {
		return fmt.Errorf("missing required field 'type'")
	}

	typeInterface := component.Get("type")

	typeString, ok := typeInterface.(string)
	if !ok {
		return fmt.Errorf("non-string type %T for field 'type'", typeInterface)
	}

	builderFunc, ok := DefaultRegistry.Lookup(typeString)
	if !ok {
		return fmt.Errorf("unsupported type '%s'", typeString)
	}

	builder := builderFunc()
	if err := component.Unmarshal(builder, confmap.WithErrorUnused()); err != nil {
		return fmt.Errorf("unmarshal to %s: %w", typeString, err)
	}

	c.Builder = builder
	return nil
}
