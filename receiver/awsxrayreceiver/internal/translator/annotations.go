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

package translator // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsxrayreceiver/internal/translator"

import "go.opentelemetry.io/collector/pdata/pcommon"

func addAnnotations(annos map[string]interface{}, attrs pcommon.Map) {
	for k, v := range annos {
		switch t := v.(type) {
		case int:
			attrs.PutInt(k, int64(t))
		case int32:
			attrs.PutInt(k, int64(t))
		case int64:
			attrs.PutInt(k, t)
		case string:
			attrs.PutString(k, t)
		case bool:
			attrs.PutBool(k, t)
		case float32:
			attrs.PutDouble(k, float64(t))
		case float64:
			attrs.PutDouble(k, t)
		default:
		}
	}
}
