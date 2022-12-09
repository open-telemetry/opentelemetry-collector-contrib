// Copyright  The OpenTelemetry Authors
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

package sort // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/comparetest/sort"

import (
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"
)

// Assumes a.Sort() and b.Sort() have been called previously
// 0: same
// -: a less than b
// +: a greater than b
func compareMaps(a, b pcommon.Map) int {
	if a.Len() != b.Len() {
		return a.Len() - b.Len()
	}

	var aKeys, bKeys []string
	a.Range(func(k string, _ pcommon.Value) bool {
		aKeys = append(aKeys, k)
		return true
	})
	b.Range(func(k string, _ pcommon.Value) bool {
		bKeys = append(bKeys, k)
		return true
	})

	for i := 0; i < len(aKeys); i++ {
		if aKeys[i] != bKeys[i] {
			return strings.Compare(aKeys[i], bKeys[i])
		}
	}

	for _, k := range aKeys {
		av, _ := a.Get(k)
		bv, _ := b.Get(k)
		if e := compareValues(av, bv); e != 0 {
			return e
		}
	}

	return 0
}

// Assumes a.Sort() and b.Sort() have been called previously
// 0: same
// -: a less than b
// +: a greater than b
func compareSlices(a, b pcommon.Slice) int {
	if a.Len() != b.Len() {
		return a.Len() - b.Len()
	}
	for i := 0; i < a.Len(); i++ {
		if e := compareValues(a.At(i), b.At(i)); e != 0 {
			return e
		}
	}
	return 0
}

func compareValues(a, b pcommon.Value) int {
	if a.Type() != b.Type() {
		return int(a.Type()) - int(b.Type())
	}

	switch a.Type() {
	case pcommon.ValueTypeBool:
		switch {
		case a.Bool() == b.Bool():
			return 0
		case !a.Bool():
			return -1
		case !b.Bool():
			return 1
		}
	case pcommon.ValueTypeBytes:
		return strings.Compare(string(a.Bytes().AsRaw()), string(b.Bytes().AsRaw()))
	case pcommon.ValueTypeDouble:
		switch {
		case a.Double() == b.Double():
			return 0
		case a.Double() < b.Double():
			return -1
		case a.Double() > b.Double():
			return 1
		}
	case pcommon.ValueTypeInt:
		return int(a.Int() - b.Int())
	case pcommon.ValueTypeMap:
		return compareMaps(a.Map(), b.Map())
	case pcommon.ValueTypeSlice:
		return compareSlices(a.Slice(), b.Slice())
	case pcommon.ValueTypeStr:
		return strings.Compare(a.Str(), b.Str())
	}
	return 0
}
