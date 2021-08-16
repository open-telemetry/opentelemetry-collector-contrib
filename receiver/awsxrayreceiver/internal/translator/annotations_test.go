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
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/model/pdata"
)

func TestAddAnnotations(t *testing.T) {
	input := make(map[string]interface{})
	input["int"] = 0
	input["int32"] = int32(1)
	input["int64"] = int64(2)
	input["bool"] = false
	input["float32"] = float32(4.5)
	input["float64"] = 5.5

	attrMap := pdata.NewAttributeMap()
	attrMap.EnsureCapacity(initAttrCapacity)
	addAnnotations(input, &attrMap)

	expectedAttrMap := pdata.NewAttributeMap()
	expectedAttrMap.InitFromMap(
		map[string]pdata.AttributeValue{
			"int":     pdata.NewAttributeValueInt(int64(0)),
			"int32":   pdata.NewAttributeValueInt(int64(1)),
			"int64":   pdata.NewAttributeValueInt(int64(2)),
			"bool":    pdata.NewAttributeValueBool(false),
			"float32": pdata.NewAttributeValueDouble(4.5),
			"float64": pdata.NewAttributeValueDouble(5.5),
		},
	)
	assert.Equal(t, expectedAttrMap.Sort(), attrMap.Sort(), "attribute maps differ")
}
