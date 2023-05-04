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

package dimensions // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter/internal/dimensions"

import (
	"fmt"
	"strings"
)

type DimensionUpdate struct {
	Name       string
	Value      string
	Properties map[string]*string
	Tags       map[string]bool
}

func (d *DimensionUpdate) String() string {
	props := "{"
	for k, v := range d.Properties {
		var val string
		if v != nil {
			val = *v
		}
		props += fmt.Sprintf("%v: %q, ", k, val)
	}
	props = strings.Trim(props, ", ") + "}"
	return fmt.Sprintf("{name: %q; value: %q; props: %v; tags: %v}", d.Name, d.Value, props, d.Tags)
}

func (d *DimensionUpdate) Key() DimensionKey {
	return DimensionKey{
		Name:  d.Name,
		Value: d.Value,
	}
}

// DimensionKey is what uniquely identifies a dimension, its name and value
// together.
type DimensionKey struct {
	Name  string
	Value string
}

func (dk DimensionKey) String() string {
	return fmt.Sprintf("[%s/%s]", dk.Name, dk.Value)
}
