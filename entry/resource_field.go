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

// ResourceField is the path to an entry's resource key
type ResourceField struct {
	key string
}

// Get will return the resource value and a boolean indicating if it exists
func (r ResourceField) Get(entry *Entry) (interface{}, bool) {
	if entry.Resource == nil {
		return "", false
	}
	val, ok := entry.Resource[r.key]
	return val, ok
}

// Set will set the resource value on an entry
func (r ResourceField) Set(entry *Entry, val interface{}) error {
	if entry.Resource == nil {
		entry.Resource = make(map[string]interface{}, 1)
	}

	str, ok := val.(string)
	if !ok {
		return fmt.Errorf("cannot set a resource to a non-string value")
	}
	entry.Resource[r.key] = str
	return nil
}

// Delete will delete a resource key from an entry
func (r ResourceField) Delete(entry *Entry) (interface{}, bool) {
	if entry.Resource == nil {
		return "", false
	}

	val, ok := entry.Resource[r.key]
	delete(entry.Resource, r.key)
	return val, ok
}

func (r ResourceField) String() string {
	if strings.Contains(r.key, ".") {
		return fmt.Sprintf(`resource['%s']`, r.key)
	}
	return "resource." + r.key
}

// NewResourceField will creat a new resource field from a key
func NewResourceField(key string) Field {
	return Field{ResourceField{key}}
}
