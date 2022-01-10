// Copyright  The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package keybuilder // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/spanmetricsprocessor/keybuilder"

import "strings"

const (
	separator       = string(byte(0))
	defaultCapacity = 1024
)

type KeyBuilder interface {
	Append(value ...string)
	String() string
}

type keyBuilder struct {
	sb        strings.Builder
	separator string
}

var _ KeyBuilder = (*keyBuilder)(nil)

func New() KeyBuilder {
	b := keyBuilder{
		sb:        strings.Builder{},
		separator: separator,
	}
	b.sb.Grow(defaultCapacity)
	return &b
}

func (mkb *keyBuilder) Append(values ...string) {
	for _, value := range values {
		if len(value) == 0 {
			continue
		}
		if mkb.sb.Len() != 0 {
			mkb.sb.WriteString(mkb.separator)
		}
		mkb.sb.WriteString(value)
	}
}

func (mkb *keyBuilder) String() string {
	return mkb.sb.String()
}
