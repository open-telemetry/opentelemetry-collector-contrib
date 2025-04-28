// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package entry // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"

import (
	"encoding/json"
	"errors"
	"fmt"
)

// AttributeField is the path to an entry attribute
type AttributeField struct {
	Keys []string
}

// NewAttributeField will creat a new attribute field from a key
func NewAttributeField(keys ...string) Field {
	if keys == nil {
		keys = []string{}
	}
	return Field{AttributeField{
		Keys: keys,
	}}
}

// Parent returns the parent of the current field.
// In the case that the attribute field points to the root node, it is a no-op.
func (f AttributeField) Parent() AttributeField {
	if f.isRoot() {
		return f
	}

	keys := f.Keys[:len(f.Keys)-1]
	return AttributeField{keys}
}

// Child returns a child of the current field using the given key.
func (f AttributeField) Child(key string) AttributeField {
	child := make([]string, len(f.Keys), len(f.Keys)+1)
	copy(child, f.Keys)
	child = append(child, key)
	return AttributeField{child}
}

// IsRoot returns a boolean indicating if this is a root level field.
func (f AttributeField) isRoot() bool {
	return len(f.Keys) == 0
}

// String returns the string representation of this field.
func (f AttributeField) String() string {
	return toJSONDot(AttributesPrefix, f.Keys)
}

// Get will return the attribute value and a boolean indicating if it exists
func (f AttributeField) Get(entry *Entry) (any, bool) {
	if entry.Attributes == nil {
		return "", false
	}

	if f.isRoot() {
		return entry.Attributes, true
	}

	currentValue, ok := entry.Attributes[f.Keys[0]]
	if !ok {
		return nil, false
	}

	for _, key := range f.Keys[1:] {
		currentMap, ok := currentValue.(map[string]any)
		if !ok {
			return nil, false
		}

		currentValue, ok = currentMap[key]
		if !ok {
			return nil, false
		}
	}

	return currentValue, true
}

// Set will set a value on an entry's attributes using the field.
// If a key already exists, it will be overwritten.
func (f AttributeField) Set(entry *Entry, value any) error {
	if entry.Attributes == nil {
		entry.Attributes = map[string]any{}
	}

	mapValue, isMapValue := value.(map[string]any)
	if isMapValue {
		f.Merge(entry, mapValue)
		return nil
	}

	if f.isRoot() {
		return errors.New("cannot set attributes root")
	}

	currentMap := entry.Attributes
	for i, key := range f.Keys {
		if i == len(f.Keys)-1 {
			currentMap[key] = value
			break
		}
		currentMap = getNestedMap(currentMap, key)
	}
	return nil
}

// Merge will attempt to merge the contents of a map into an entry's attributes.
// It will overwrite any intermediate values as necessary.
func (f AttributeField) Merge(entry *Entry, mapValues map[string]any) {
	currentMap := entry.Attributes

	for _, key := range f.Keys {
		currentMap = getNestedMap(currentMap, key)
	}

	for key, value := range mapValues {
		currentMap[key] = value
	}
}

// Delete removes a value from an entry's attributes using the field.
// It will return the deleted value and whether the field existed.
func (f AttributeField) Delete(entry *Entry) (any, bool) {
	if entry.Attributes == nil {
		return "", false
	}

	if f.isRoot() {
		oldAttributes := entry.Attributes
		entry.Attributes = nil
		return oldAttributes, true
	}

	currentMap := entry.Attributes
	for i, key := range f.Keys {
		currentValue, ok := currentMap[key]
		if !ok {
			break
		}

		if i == len(f.Keys)-1 {
			delete(currentMap, key)
			return currentValue, true
		}

		currentMap, ok = currentValue.(map[string]any)
		if !ok {
			break
		}
	}

	return nil, false
}

/****************
  Serialization
****************/

// UnmarshalJSON will attempt to unmarshal the field from JSON.
func (f *AttributeField) UnmarshalJSON(raw []byte) error {
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return fmt.Errorf("the field is not a string: %w", err)
	}

	keys, err := fromJSONDot(value)
	if err != nil {
		return err
	}

	if keys[0] != AttributesPrefix {
		return fmt.Errorf("must start with 'attributes': %s", value)
	}

	*f = AttributeField{keys[1:]}
	return nil
}

// UnmarshalYAML will attempt to unmarshal a field from YAML.
func (f *AttributeField) UnmarshalYAML(unmarshal func(any) error) error {
	var value string
	if err := unmarshal(&value); err != nil {
		return fmt.Errorf("the field is not a string: %w", err)
	}

	keys, err := fromJSONDot(value)
	if err != nil {
		return err
	}

	if keys[0] != AttributesPrefix {
		return fmt.Errorf("must start with 'attributes': %s", value)
	}

	*f = AttributeField{keys[1:]}
	return nil
}

// UnmarshalText will unmarshal a field from text
func (f *AttributeField) UnmarshalText(text []byte) error {
	keys, err := fromJSONDot(string(text))
	if err != nil {
		return err
	}

	if keys[0] != AttributesPrefix {
		return fmt.Errorf("must start with 'attributes': %s", text)
	}

	*f = AttributeField{keys[1:]}
	return nil
}
