// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package operator // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"

import (
	"encoding/json"
	"fmt"

	"go.opentelemetry.io/collector/confmap"
	"go.uber.org/zap"
)

// Deprecated [v0.97.0] Use Identity and GlobalFactoryRegistry instead.
type Config struct {
	Builder
}

// Deprecated [v0.97.0] Use Identity and GlobalFactoryRegistry instead.
func NewConfig(b Builder) Config {
	return Config{Builder: b}
}

// Deprecated [v0.97.0] Use Identity and GlobalFactoryRegistry instead.
type Builder interface {
	ID() string
	Type() string
	Build(*zap.SugaredLogger) (Operator, error)
	SetID(string)
}

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

func (c *Config) UnmarshalYAML(unmarshal func(any) error) error {
	rawConfig := map[string]any{}
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
	if err := component.Unmarshal(builder, confmap.WithIgnoreUnused()); err != nil {
		return fmt.Errorf("unmarshal to %s: %w", typeString, err)
	}

	c.Builder = builder
	return nil
}
