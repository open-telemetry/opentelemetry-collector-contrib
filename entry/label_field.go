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

// LabelField is the path to an entry label
type LabelField struct {
	key string
}

// Get will return the label value and a boolean indicating if it exists
func (l LabelField) Get(entry *Entry) (interface{}, bool) {
	if entry.Labels == nil {
		return "", false
	}
	val, ok := entry.Labels[l.key]
	return val, ok
}

// Set will set the label value on an entry
func (l LabelField) Set(entry *Entry, val interface{}) error {
	if entry.Labels == nil {
		entry.Labels = make(map[string]string, 1)
	}

	str, ok := val.(string)
	if !ok {
		return fmt.Errorf("cannot set a label to a non-string value")
	}
	entry.Labels[l.key] = str
	return nil
}

// Delete will delete a label from an entry
func (l LabelField) Delete(entry *Entry) (interface{}, bool) {
	if entry.Labels == nil {
		return "", false
	}

	val, ok := entry.Labels[l.key]
	delete(entry.Labels, l.key)
	return val, ok
}

func (l LabelField) String() string {
	if strings.Contains(l.key, ".") {
		return fmt.Sprintf(`$labels['%s']`, l.key)
	}
	return "$labels." + l.key
}

// NewLabelField will creat a new label field from a key
func NewLabelField(key string) Field {
	return Field{LabelField{key}}
}
