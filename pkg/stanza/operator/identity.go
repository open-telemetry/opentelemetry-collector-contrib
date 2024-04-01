// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package operator // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"

import (
	"encoding/json"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
)

// // Identifiable is an interface for obtaining operator type and name.
// type Identifiable interface {
// 	ComponentID() component.ID
// 	SetName(string)
// }

// var _ Identifiable = (*Identity)(nil)

// ConfigWithID provides a basic implemention for an operator config.
type ConfigWithID struct {
	Type string `mapstructure:"type"`
	Name string `mapstructure:"id"` // "id" is the legacy key used in user configs.
	factory
}

// ID returns the component ID for the operator.
func (i *ConfigWithID) ComponentID() component.ID {
	return component.MustNewIDWithName(i.Type, i.Name)
}

func (i *ConfigWithID) SetName(name string) {
	i.Name = name
}

// UnmarshalJSON will unmarshal a config from JSON.
func (i *ConfigWithID) UnmarshalJSON(bytes []byte) error {
	var typeUnmarshaller struct {
		Type string
	}
	if err := json.Unmarshal(bytes, &typeUnmarshaller); err != nil {
		return err
	}
	if typeUnmarshaller.Type == "" {
		return fmt.Errorf("missing required field 'type'")
	}
	_, err := getDefaultConfig(typeUnmarshaller.Type)
	return err
}

// UnmarshalYAML will unmarshal a config from YAML.
func (i *ConfigWithID) UnmarshalYAML(unmarshal func(any) error) error {
	raw := map[string]any{}
	err := unmarshal(&raw)
	if err != nil {
		return fmt.Errorf("failed to unmarshal yaml to base config: %w", err)
	}
	typeInterface, ok := raw["type"]
	if !ok {
		return fmt.Errorf("missing required field 'type'")
	}
	typeString, ok := typeInterface.(string)
	if !ok {
		return fmt.Errorf("non-string type %T for field 'type'", typeInterface)
	}
	_, err = getDefaultConfig(typeString)
	return err
}

func (i *ConfigWithID) Unmarshal(conf *confmap.Conf) error {
	if !conf.IsSet("type") {
		return fmt.Errorf("missing required field 'type'")
	}
	typeInterface := conf.Get("type")
	typeString, ok := typeInterface.(string)
	if !ok {
		return fmt.Errorf("non-string type %T for field 'type'", typeInterface)
	}
	cfg, err := getDefaultConfig(typeString)
	if err != nil {
		return err
	}
	if err := conf.Unmarshal(cfg, confmap.WithIgnoreUnused()); err != nil {
		return fmt.Errorf("unmarshal to %s: %w", typeString, err)
	}
	return nil
}

func getDefaultConfig(t string) (component.Config, error) {
	ct, err := component.NewType(t)
	if err != nil {
		return nil, fmt.Errorf("invalid type '%s'", t)
	}
	f, ok := GlobalFactoryRegistry.LookupFactory(ct)
	if !ok {
		return nil, fmt.Errorf("unsupported operator type '%s'", t)
	}
	return f.NewDefaultConfig(""), nil
}
