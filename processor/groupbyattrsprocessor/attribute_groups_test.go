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

	"go.opentelemetry.io/collector/consumer/pdata"
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
	rs       = simpleResource()
	lagAttrs = &logsGroupedByAttrs{}
)

func BenchmarkAttrs(b *testing.B) {
	lagAttrs.attributeGroup(groups[rand.Intn(count)], rs)
}
