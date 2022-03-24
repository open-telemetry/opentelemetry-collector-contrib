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
	"fmt"
	"strings"
)

// AttributeField is the path to an entry attribute
type AttributeField struct {
	key string
}

// Get will return the attribute value and a boolean indicating if it exists
func (l AttributeField) Get(entry *Entry) (interface{}, bool) {
	if entry.Attributes == nil {
		return "", false
	}
	val, ok := entry.Attributes[l.key]
	return val, ok
}

// Set will set the attribute value on an entry
func (l AttributeField) Set(entry *Entry, val interface{}) error {
	if entry.Attributes == nil {
		entry.Attributes = make(map[string]interface{}, 1)
	}

	str, ok := val.(string)
	if !ok {
		return fmt.Errorf("cannot set a attribute to a non-string value")
	}
	entry.Attributes[l.key] = str
	return nil
}

// Delete will delete a attribute from an entry
func (l AttributeField) Delete(entry *Entry) (interface{}, bool) {
	if entry.Attributes == nil {
		return "", false
	}

	val, ok := entry.Attributes[l.key]
	delete(entry.Attributes, l.key)
	return val, ok
}

func (l AttributeField) String() string {
	if strings.Contains(l.key, ".") {
		return fmt.Sprintf(`attributes['%s']`, l.key)
	}
	return "attributes." + l.key
}

// NewAttributeField will creat a new attribute field from a key
func NewAttributeField(key string) Field {
	return Field{AttributeField{key}}
}
