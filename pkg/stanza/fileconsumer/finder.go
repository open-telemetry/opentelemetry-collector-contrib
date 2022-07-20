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

package fileconsumer // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer"

import (
	"github.com/bmatcuk/doublestar/v3"
)

type Finder struct {
	Include []string `mapstructure:"include,omitempty" json:"include,omitempty" yaml:"include,omitempty"`
	Exclude []string `mapstructure:"exclude,omitempty" json:"exclude,omitempty" yaml:"exclude,omitempty"`
}

// FindFiles gets a list of paths given an array of glob patterns to include and exclude
func (f Finder) FindFiles() []string {
	all := make([]string, 0, len(f.Include))
	for _, include := range f.Include {
		matches, _ := doublestar.Glob(include) // compile error checked in build
	INCLUDE:
		for _, match := range matches {
			for _, exclude := range f.Exclude {
				if itMatches, _ := doublestar.PathMatch(exclude, match); itMatches {
					continue INCLUDE
				}
			}

			for _, existing := range all {
				if existing == match {
					continue INCLUDE
				}
			}

			all = append(all, match)
		}
	}

	return all
}
