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

package entry // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"

// NilField is a struct that implements Field, but
// does nothing for all its operations. It is useful
// as a default no-op field to avoid nil checks.
type NilField struct{}

// Get will return always return nil
func (l NilField) Get(entry *Entry) (interface{}, bool) {
	return nil, true
}

// Set will do nothing and return no error
func (l NilField) Set(entry *Entry, val interface{}) error {
	return nil
}

// Delete will do nothing and return no error
func (l NilField) Delete(entry *Entry) (interface{}, bool) {
	return nil, true
}

func (l NilField) String() string {
	return "$nil"
}

// NewNilField will create a new nil field
func NewNilField() Field {
	return Field{NilField{}}
}
