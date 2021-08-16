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
	"encoding/json"

	"go.opentelemetry.io/collector/model/pdata"

	awsxray "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/xray"
)

func addMetadata(meta map[string]map[string]interface{}, attrs *pdata.AttributeMap) error {
	for k, v := range meta {
		val, err := json.Marshal(v)
		if err != nil {
			return err
		}
		attrs.UpsertString(
			awsxray.AWSXraySegmentMetadataAttributePrefix+k, string(val))
	}
	return nil
}
