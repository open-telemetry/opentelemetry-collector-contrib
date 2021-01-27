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

package entry

import (
	"encoding/json"
	"fmt"
	"strings"
)

// RecordField is a field found on an entry record.
type RecordField struct {
	Keys []string
}

// Parent returns the parent of the current field.
// In the case that the record field points to the root node, it is a no-op.
func (f RecordField) Parent() RecordField {
	if f.isRoot() {
		return f
	}

	keys := f.Keys[:len(f.Keys)-1]
	return RecordField{keys}
}

// Child returns a child of the current field using the given key.
func (f RecordField) Child(key string) RecordField {
	child := make([]string, len(f.Keys), len(f.Keys)+1)
	copy(child, f.Keys)
	keys := append(child, key)
	return RecordField{keys}
}

// IsRoot returns a boolean indicating if this is a root level field.
func (f RecordField) isRoot() bool {
	return len(f.Keys) == 0
}

// String returns the string representation of this field.
func (f RecordField) String() string {
	return toJSONDot(f)
}

// Get will retrieve a value from an entry's record using the field.
// It will return the value and whether the field existed.
func (f RecordField) Get(entry *Entry) (interface{}, bool) {
	var currentValue interface{} = entry.Record

	for _, key := range f.Keys {
		currentRecord, ok := currentValue.(map[string]interface{})
		if !ok {
			return nil, false
		}

		currentValue, ok = currentRecord[key]
		if !ok {
			return nil, false
		}
	}

	return currentValue, true
}

// Set will set a value on an entry's record using the field.
// If a key already exists, it will be overwritten.
// If mergeMaps is set to true, map values will be merged together.
func (f RecordField) Set(entry *Entry, value interface{}) error {
	mapValue, isMapValue := value.(map[string]interface{})
	if isMapValue {
		f.Merge(entry, mapValue)
		return nil
	}

	if f.isRoot() {
		entry.Record = value
		return nil
	}

	currentMap, ok := entry.Record.(map[string]interface{})
	if !ok {
		currentMap = map[string]interface{}{}
		entry.Record = currentMap
	}

	for i, key := range f.Keys {
		if i == len(f.Keys)-1 {
			currentMap[key] = value
			break
		}
		currentMap = f.getNestedMap(currentMap, key)
	}
	return nil
}

// Merge will attempt to merge the contents of a map into an entry's record.
// It will overwrite any intermediate values as necessary.
func (f RecordField) Merge(entry *Entry, mapValues map[string]interface{}) {
	currentMap, ok := entry.Record.(map[string]interface{})
	if !ok {
		currentMap = map[string]interface{}{}
		entry.Record = currentMap
	}

	for _, key := range f.Keys {
		currentMap = f.getNestedMap(currentMap, key)
	}

	for key, value := range mapValues {
		currentMap[key] = value
	}
}

// Delete removes a value from an entry's record using the field.
// It will return the deleted value and whether the field existed.
func (f RecordField) Delete(entry *Entry) (interface{}, bool) {
	if f.isRoot() {
		oldRecord := entry.Record
		entry.Record = nil
		return oldRecord, true
	}

	currentValue := entry.Record
	for i, key := range f.Keys {
		currentMap, ok := currentValue.(map[string]interface{})
		if !ok {
			break
		}

		currentValue, ok = currentMap[key]
		if !ok {
			break
		}

		if i == len(f.Keys)-1 {
			delete(currentMap, key)
			return currentValue, true
		}
	}

	return nil, false
}

// getNestedMap will get a nested map assigned to a key.
// If the map does not exist, it will create and return it.
func (f RecordField) getNestedMap(currentMap map[string]interface{}, key string) map[string]interface{} {
	currentValue, ok := currentMap[key]
	if !ok {
		currentMap[key] = map[string]interface{}{}
	}

	nextMap, ok := currentValue.(map[string]interface{})
	if !ok {
		nextMap = map[string]interface{}{}
		currentMap[key] = nextMap
	}

	return nextMap
}

/****************
  Serialization
****************/

// UnmarshalJSON will attempt to unmarshal the field from JSON.
func (f *RecordField) UnmarshalJSON(raw []byte) error {
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return fmt.Errorf("the field is not a string: %s", err)
	}

	*f = fromJSONDot(value)
	return nil
}

// MarshalJSON will marshal the field for JSON.
func (f RecordField) MarshalJSON() ([]byte, error) {
	json := fmt.Sprintf(`"%s"`, toJSONDot(f))
	return []byte(json), nil
}

// UnmarshalYAML will attempt to unmarshal a field from YAML.
func (f *RecordField) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var value string
	if err := unmarshal(&value); err != nil {
		return fmt.Errorf("the field is not a string: %s", err)
	}

	*f = fromJSONDot(value)
	return nil
}

// MarshalYAML will marshal the field for YAML.
func (f RecordField) MarshalYAML() (interface{}, error) {
	return toJSONDot(f), nil
}

// fromJSONDot creates a field from JSON dot notation.
func fromJSONDot(value string) RecordField {
	keys := strings.Split(value, ".")

	if keys[0] == "$" || keys[0] == recordPrefix {
		keys = keys[1:]
	}

	return RecordField{keys}
}

// toJSONDot returns the JSON dot notation for a field.
func toJSONDot(field RecordField) string {
	if field.isRoot() {
		return recordPrefix
	}

	containsDots := false
	for _, key := range field.Keys {
		if strings.Contains(key, ".") {
			containsDots = true
		}
	}

	var b strings.Builder
	if containsDots {
		b.WriteString(recordPrefix)
		for _, key := range field.Keys {
			b.WriteString(`['`)
			b.WriteString(key)
			b.WriteString(`']`)
		}
	} else {
		for i, key := range field.Keys {
			if i != 0 {
				b.WriteString(".")
			}
			b.WriteString(key)
		}
	}

	return b.String()
}

// NewRecordField creates a new field from an ordered array of keys.
func NewRecordField(keys ...string) Field {
	return Field{RecordField{
		Keys: keys,
	}}
}
