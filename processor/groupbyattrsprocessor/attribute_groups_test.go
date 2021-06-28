// Copyright 2020 OpenTelemetry Authors
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

package groupbyattrsprocessor

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/model/pdata"
)

func simpleResource() pdata.Resource {
	rs := pdata.NewResource()
	rs.Attributes().Insert("somekey1", pdata.NewAttributeValueString("some-string-value"))
	rs.Attributes().Insert("somekey2", pdata.NewAttributeValueInt(123))
	for i := 0; i < 10; i++ {
		k := fmt.Sprint("random-", i)
		v := fmt.Sprint("value-", rand.Intn(100))
		rs.Attributes().Insert(k, pdata.NewAttributeValueString(v))
	}
	return rs
}

func randomAttributeMap() pdata.AttributeMap {
	attrs := pdata.NewAttributeMap()
	for i := 0; i < 10; i++ {
		k := fmt.Sprint("key-", i)
		v := fmt.Sprint("value-", rand.Intn(500000))
		attrs.InsertString(k, v)
	}
	return attrs
}

func randomGroups(count int) []pdata.AttributeMap {
	entries := make([]pdata.AttributeMap, count)
	for i := 0; i < count; i++ {
		entries[i] = randomAttributeMap()
	}
	return entries
}

var (
	count    = 1000
	groups   = randomGroups(count)
	res      = simpleResource()
	lagAttrs = newLogsGroupedByAttrs()
)

func TestResourceAttributeScenarios(t *testing.T) {
	tests := []struct {
		name                    string
		baseResource            pdata.Resource
		fillRecordAttributesFun func(attributeMap pdata.AttributeMap)
		fillExpectedResourceFun func(baseResource pdata.Resource, expectedResource pdata.Resource)
	}{
		{
			name:         "When the same key is present at Resource and Record level, the latter value should be used",
			baseResource: simpleResource(),
			fillRecordAttributesFun: func(attributeMap pdata.AttributeMap) {
				attributeMap.InsertString("somekey1", "replaced-value")
			},
			fillExpectedResourceFun: func(baseResource pdata.Resource, expectedResource pdata.Resource) {
				baseResource.CopyTo(expectedResource)
				expectedResource.Attributes().UpdateString("somekey1", "replaced-value")
			},
		},
		{
			name:                    "Empty Resource and attributes",
			baseResource:            pdata.NewResource(),
			fillRecordAttributesFun: nil,
			fillExpectedResourceFun: nil,
		},
		{
			name:         "Empty Resource",
			baseResource: pdata.NewResource(),
			fillRecordAttributesFun: func(attributeMap pdata.AttributeMap) {
				attributeMap.InsertString("somekey1", "some-value")
			},
			fillExpectedResourceFun: func(_ pdata.Resource, expectedResource pdata.Resource) {
				expectedResource.Attributes().InsertString("somekey1", "some-value")
			},
		},
		{
			name:                    "Empty Attributes",
			baseResource:            simpleResource(),
			fillRecordAttributesFun: nil,
			fillExpectedResourceFun: func(baseResource pdata.Resource, expectedResource pdata.Resource) {
				baseResource.CopyTo(expectedResource)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recordAttributeMap := pdata.NewAttributeMap()
			if tt.fillRecordAttributesFun != nil {
				tt.fillRecordAttributesFun(recordAttributeMap)
			}

			expectedResource := pdata.NewResource()
			if tt.fillExpectedResourceFun != nil {
				tt.fillExpectedResourceFun(tt.baseResource, expectedResource)
			}

			rl := lagAttrs.attributeGroup(tt.baseResource, recordAttributeMap)
			assert.EqualValues(t, expectedResource.Attributes(), rl.Resource().Attributes())
		})
	}
}

func TestInstrumentationLibraryMatching(t *testing.T) {
	rl := pdata.NewResourceLogs()
	rs := pdata.NewResourceSpans()

	il1 := pdata.NewInstrumentationLibrary()
	il1.SetName("Name1")
	il2 := pdata.NewInstrumentationLibrary()
	il2.SetName("Name2")

	ill1 := matchingInstrumentationLibraryLogs(rl, il1)
	ils1 := matchingInstrumentationLibrarySpans(rs, il1)
	assert.EqualValues(t, il1, ill1.InstrumentationLibrary())
	assert.EqualValues(t, il1, ils1.InstrumentationLibrary())

	ill2 := matchingInstrumentationLibraryLogs(rl, il2)
	ils2 := matchingInstrumentationLibrarySpans(rs, il2)
	assert.EqualValues(t, il2, ill2.InstrumentationLibrary())
	assert.EqualValues(t, il2, ils2.InstrumentationLibrary())

	ill1 = matchingInstrumentationLibraryLogs(rl, il1)
	ils1 = matchingInstrumentationLibrarySpans(rs, il1)
	assert.EqualValues(t, il1, ill1.InstrumentationLibrary())
	assert.EqualValues(t, il1, ils1.InstrumentationLibrary())
}

func BenchmarkAttrGrouping(b *testing.B) {
	lagAttrs.attributeGroup(res, groups[rand.Intn(count)])
}
