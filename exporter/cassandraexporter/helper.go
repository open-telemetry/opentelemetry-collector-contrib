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

package cassandraexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/cassandraexporter"

import "encoding/json"

func attributesToMap(attributes map[string]any) map[string]string {
	newAttrMap := make(map[string]string)
	for k, v := range attributes {
		jsonV, err := json.Marshal(v)
		if err == nil {
			newAttrMap[k] = string(jsonV)
		}
	}
	return newAttrMap
}
