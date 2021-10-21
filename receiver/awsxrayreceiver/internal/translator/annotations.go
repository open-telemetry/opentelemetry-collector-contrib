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

package translator

import (
	"go.opentelemetry.io/collector/model/pdata"
)

func addAnnotations(annos map[string]interface{}, attrs *pdata.AttributeMap) {
	for k, v := range annos {
		switch t := v.(type) {
		case int:
			attrs.UpsertInt(k, int64(t))
		case int32:
			attrs.UpsertInt(k, int64(t))
		case int64:
			attrs.UpsertInt(k, t)
		case string:
			attrs.UpsertString(k, t)
		case bool:
			attrs.UpsertBool(k, t)
		case float32:
			attrs.UpsertDouble(k, float64(t))
		case float64:
			attrs.UpsertDouble(k, t)
		default:
		}
	}
}
