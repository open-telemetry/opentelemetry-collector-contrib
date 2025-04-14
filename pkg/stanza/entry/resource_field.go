// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package entry // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"

import (
	"encoding/json"
	"errors"
	"fmt"
)

// ResourceField is the path to an entry resource
type ResourceField struct {
	Keys []string
}

// NewResourceField will creat a new resource field from a key
func NewResourceField(keys ...string) Field {
	if keys == nil {
		keys = []string{}
	}
	return Field{ResourceField{
		Keys: keys,
	}}
}

// Parent returns the parent of the current field.
// In the case that the resource field points to the root node, it is a no-op.
func (f ResourceField) Parent() ResourceField {
	if f.isRoot() {
		return f
	}

	keys := f.Keys[:len(f.Keys)-1]
	return ResourceField{keys}
}

// Child returns a child of the current field using the given key.
func (f ResourceField) Child(key string) ResourceField {
	child := make([]string, len(f.Keys), len(f.Keys)+1)
	copy(child, f.Keys)
	child = append(child, key)
	return ResourceField{child}
}

// IsRoot returns a boolean indicating if this is a root level field.
func (f ResourceField) isRoot() bool {
	return len(f.Keys) == 0
}

// String returns the string representation of this field.
func (f ResourceField) String() string {
	return toJSONDot(ResourcePrefix, f.Keys)
}

// Get will return the resource value and a boolean indicating if it exists
func (f ResourceField) Get(entry *Entry) (any, bool) {
	if entry.Resource == nil {
		return "", false
	}

	if f.isRoot() {
		return entry.Resource, true
	}

	currentValue, ok := entry.Resource[f.Keys[0]]
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

// Set will set a value on an entry's resource using the field.
// If a key already exists, it will be overwritten.
func (f ResourceField) Set(entry *Entry, value any) error {
	if entry.Resource == nil {
		entry.Resource = map[string]any{}
	}

	mapValue, isMapValue := value.(map[string]any)
	if isMapValue {
		f.Merge(entry, mapValue)
		return nil
	}

	if f.isRoot() {
		return errors.New("cannot set resource root")
	}

	currentMap := entry.Resource
	for i, key := range f.Keys {
		if i == len(f.Keys)-1 {
			currentMap[key] = value
			break
		}
		currentMap = getNestedMap(currentMap, key)
	}
	return nil
}

// Merge will attempt to merge the contents of a map into an entry's resource.
// It will overwrite any intermediate values as necessary.
func (f ResourceField) Merge(entry *Entry, mapValues map[string]any) {
	currentMap := entry.Resource

	for _, key := range f.Keys {
		currentMap = getNestedMap(currentMap, key)
	}

	for key, value := range mapValues {
		currentMap[key] = value
	}
}

// Delete removes a value from an entry's resource using the field.
// It will return the deleted value and whether the field existed.
func (f ResourceField) Delete(entry *Entry) (any, bool) {
	if entry.Resource == nil {
		return "", false
	}

	if f.isRoot() {
		oldResource := entry.Resource
		entry.Resource = nil
		return oldResource, true
	}

	currentMap := entry.Resource
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
func (f *ResourceField) UnmarshalJSON(raw []byte) error {
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return fmt.Errorf("the field is not a string: %w", err)
	}

	keys, err := fromJSONDot(value)
	if err != nil {
		return err
	}

	if keys[0] != ResourcePrefix {
		return fmt.Errorf("must start with 'resource': %s", value)
	}

	*f = ResourceField{keys[1:]}
	return nil
}

// UnmarshalYAML will attempt to unmarshal a field from YAML.
func (f *ResourceField) UnmarshalYAML(unmarshal func(any) error) error {
	var value string
	if err := unmarshal(&value); err != nil {
		return fmt.Errorf("the field is not a string: %w", err)
	}

	keys, err := fromJSONDot(value)
	if err != nil {
		return err
	}

	if keys[0] != ResourcePrefix {
		return fmt.Errorf("must start with 'resource': %s", value)
	}

	*f = ResourceField{keys[1:]}
	return nil
}

// UnmarshalText will unmarshal a field from text
func (f *ResourceField) UnmarshalText(text []byte) error {
	keys, err := fromJSONDot(string(text))
	if err != nil {
		return err
	}

	if keys[0] != ResourcePrefix {
		return fmt.Errorf("must start with 'resource': %s", text)
	}

	*f = ResourceField{keys[1:]}
	return nil
}
