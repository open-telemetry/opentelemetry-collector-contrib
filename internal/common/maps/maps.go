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

package maps // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/maps"

// MergeRawMaps merges n maps with a later map's keys overriding earlier maps.
func MergeRawMaps(maps ...map[string]interface{}) map[string]interface{} {
	ret := map[string]interface{}{}

	for _, m := range maps {
		for k, v := range m {
			ret[k] = v
		}
	}

	return ret
}

// MergeStringMaps merges n maps with a later map's keys overriding earlier maps.
func MergeStringMaps(maps ...map[string]string) map[string]string {
	ret := map[string]string{}

	for _, m := range maps {
		for k, v := range m {
			ret[k] = v
		}
	}

	return ret
}

// CloneStringMap makes a shallow copy of a map[string]string.
func CloneStringMap(m map[string]string) map[string]string {
	m2 := make(map[string]string, len(m))
	for k, v := range m {
		m2[k] = v
	}
	return m2
}
