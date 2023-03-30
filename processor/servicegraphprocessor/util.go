// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package servicegraphprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/servicegraphprocessor"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	semconv "go.opentelemetry.io/collector/semconv/v1.9.0"
)

func findAttributeValue(key string, attributes ...pcommon.Map) (string, bool) {
	for _, attr := range attributes {
		if v, ok := attr.Get(key); ok {
			return v.AsString(), true
		}
	}
	return "", false
}

func findServiceName(attributes pcommon.Map) (string, bool) {
	return findAttributeValue(semconv.AttributeServiceName, attributes)
}
