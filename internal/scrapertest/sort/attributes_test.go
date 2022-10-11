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

package sort

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func TestCompareValues(t *testing.T) {
	pTypes.test(t)
	pStrings.test(t)
	pInts.test(t)
	pDoubles.test(t)
	pBools.test(t)
	pMaps.test(t)
	pSlices.test(t)
	pBytes.test(t)
}

var pTypes ascendingValues = []pcommon.Value{
	pcommon.NewValueEmpty(),
	pcommon.NewValueString(""),
	pcommon.NewValueInt(0),
	pcommon.NewValueDouble(0),
	pcommon.NewValueBool(false),
	pcommon.NewValueMap(),
	pcommon.NewValueSlice(),
	pcommon.NewValueBytes(pcommon.NewImmutableByteSlice([]byte(""))),
}

var pBools ascendingValues = []pcommon.Value{
	pcommon.NewValueBool(false),
	pcommon.NewValueBool(true),
}

var pStrings ascendingValues = []pcommon.Value{
	pcommon.NewValueString(""),
	pcommon.NewValueString(" "),
	pcommon.NewValueString("A"),
	pcommon.NewValueString("AA"),
	pcommon.NewValueString("_"),
	pcommon.NewValueString("a"),
	pcommon.NewValueString("aa"),
	pcommon.NewValueString("b"),
	pcommon.NewValueString("long"),
	pcommon.NewValueString("z"),
}

var pBytes ascendingValues = []pcommon.Value{
	pcommon.NewValueBytes(pcommon.NewImmutableByteSlice([]byte(""))),
	pcommon.NewValueBytes(pcommon.NewImmutableByteSlice([]byte(" "))),
	pcommon.NewValueBytes(pcommon.NewImmutableByteSlice([]byte("A"))),
	pcommon.NewValueBytes(pcommon.NewImmutableByteSlice([]byte("AA"))),
	pcommon.NewValueBytes(pcommon.NewImmutableByteSlice([]byte("_"))),
	pcommon.NewValueBytes(pcommon.NewImmutableByteSlice([]byte("a"))),
	pcommon.NewValueBytes(pcommon.NewImmutableByteSlice([]byte("aa"))),
	pcommon.NewValueBytes(pcommon.NewImmutableByteSlice([]byte("b"))),
	pcommon.NewValueBytes(pcommon.NewImmutableByteSlice([]byte("long"))),
	pcommon.NewValueBytes(pcommon.NewImmutableByteSlice([]byte("z"))),
}

var pInts ascendingValues = []pcommon.Value{
	pcommon.NewValueInt(-10),
	pcommon.NewValueInt(-1),
	pcommon.NewValueInt(0),
	pcommon.NewValueInt(1),
	pcommon.NewValueInt(99),
	pcommon.NewValueInt(1000),
}

var pDoubles ascendingValues = []pcommon.Value{
	pcommon.NewValueDouble(-1.123),
	pcommon.NewValueDouble(-0.01),
	pcommon.NewValueDouble(0),
	pcommon.NewValueDouble(0.001),
	pcommon.NewValueDouble(0.01),
	pcommon.NewValueDouble(1.23),
}

// ascending lengths -> keys -> types -> values
var pMaps ascendingValues = []pcommon.Value{
	pcommon.NewValueMap(),
	func() pcommon.Value {
		v := pcommon.NewValueMap()
		m := v.SetEmptyMapVal()
		m.InsertString("a", "")
		return v
	}(),
	func() pcommon.Value {
		v := pcommon.NewValueMap()
		m := v.SetEmptyMapVal()
		m.InsertInt("a", 123)
		return v
	}(),
	func() pcommon.Value {
		v := pcommon.NewValueMap()
		m := v.SetEmptyMapVal()
		m.InsertInt("b", 100)
		return v
	}(),
	func() pcommon.Value {
		v := pcommon.NewValueMap()
		m := v.SetEmptyMapVal()
		m.InsertString("c", "")
		return v
	}(),
	func() pcommon.Value {
		v := pcommon.NewValueMap()
		m := v.SetEmptyMapVal()
		m.InsertString("a", "")
		m.InsertString("b", "")
		return v
	}(),
	func() pcommon.Value {
		v := pcommon.NewValueMap()
		m := v.SetEmptyMapVal()
		m.InsertString("a", "")
		mv := pcommon.NewValueMap()
		m.Insert("m", mv)
		return v
	}(),
	func() pcommon.Value {
		v := pcommon.NewValueMap()
		m := v.SetEmptyMapVal()
		m.InsertString("a", "")
		mv := pcommon.NewValueMap()
		mv.MapVal().InsertInt("i", 0)
		m.Insert("m", mv)
		return v
	}(),
	func() pcommon.Value {
		v := pcommon.NewValueMap()
		m := v.SetEmptyMapVal()
		m.InsertString("a", "")
		vs := pcommon.NewValueSlice()
		s := vs.SetEmptySliceVal()
		s.AppendEmpty().SetDoubleVal(100)
		m.Insert("s", vs)
		return v
	}(),
}

// ascending lengths -> types -> values
var pSlices ascendingValues = []pcommon.Value{
	pcommon.NewValueSlice(),
	func() pcommon.Value {
		v := pcommon.NewValueSlice()
		s := v.SetEmptySliceVal()
		s.AppendEmpty().SetStringVal("")
		return v
	}(),
	func() pcommon.Value {
		v := pcommon.NewValueSlice()
		s := v.SetEmptySliceVal()
		s.AppendEmpty().SetIntVal(100)
		return v
	}(),
	func() pcommon.Value {
		v := pcommon.NewValueSlice()
		s := v.SetEmptySliceVal()
		s.AppendEmpty().SetDoubleVal(100)
		return v
	}(),
	func() pcommon.Value {
		v := pcommon.NewValueSlice()
		s := v.SetEmptySliceVal()
		s.AppendEmpty().SetBoolVal(true)
		return v
	}(),
	func() pcommon.Value {
		v := pcommon.NewValueSlice()
		s := v.SetEmptySliceVal()
		s.AppendEmpty().SetEmptyMapVal()
		return v
	}(),
	func() pcommon.Value {
		v := pcommon.NewValueSlice()
		s := v.SetEmptySliceVal()
		m := s.AppendEmpty().SetEmptyMapVal()
		m.InsertString("a", "")
		return v
	}(),
	func() pcommon.Value {
		v := pcommon.NewValueSlice()
		s := v.SetEmptySliceVal()
		m := s.AppendEmpty().SetEmptyMapVal()
		m.InsertString("b", "")
		return v
	}(),
	func() pcommon.Value {
		v := pcommon.NewValueSlice()
		s := v.SetEmptySliceVal()
		s.AppendEmpty().SetEmptySliceVal()
		return v
	}(),
	func() pcommon.Value {
		v := pcommon.NewValueSlice()
		s := v.SetEmptySliceVal()
		ns := s.AppendEmpty().SetEmptySliceVal()
		ns.AppendEmpty().SetStringVal("")
		return v
	}(),
	func() pcommon.Value {
		v := pcommon.NewValueSlice()
		s := v.SetEmptySliceVal()
		s.AppendEmpty().SetBytesVal(pcommon.NewImmutableByteSlice([]byte("A")))
		return v
	}(),
	func() pcommon.Value {
		v := pcommon.NewValueSlice()
		s := v.SetEmptySliceVal()
		s.AppendEmpty().SetStringVal("")
		s.AppendEmpty().SetStringVal("a")
		return v
	}(),
	func() pcommon.Value {
		v := pcommon.NewValueSlice()
		s := v.SetEmptySliceVal()
		s.AppendEmpty().SetStringVal("")
		s.AppendEmpty().SetBoolVal(false)
		return v
	}(),
	func() pcommon.Value {
		v := pcommon.NewValueSlice()
		s := v.SetEmptySliceVal()
		s.AppendEmpty().SetIntVal(0)
		s.AppendEmpty().SetBoolVal(true)
		return v
	}(),
	func() pcommon.Value {
		v := pcommon.NewValueSlice()
		s := v.SetEmptySliceVal()
		s.AppendEmpty().SetIntVal(100)
		s.AppendEmpty().SetIntVal(0)
		return v
	}(),
	func() pcommon.Value {
		v := pcommon.NewValueSlice()
		s := v.SetEmptySliceVal()
		s.AppendEmpty().SetIntVal(100)
		s.AppendEmpty().SetDoubleVal(0)
		return v
	}(),
	func() pcommon.Value {
		v := pcommon.NewValueSlice()
		s := v.SetEmptySliceVal()
		s.AppendEmpty().SetBoolVal(false)
		s.AppendEmpty().SetBoolVal(true)
		return v
	}(),
	func() pcommon.Value {
		v := pcommon.NewValueSlice()
		s := v.SetEmptySliceVal()
		s.AppendEmpty().SetBoolVal(true)
		s.AppendEmpty().SetBoolVal(false)
		return v
	}(),
	func() pcommon.Value {
		v := pcommon.NewValueSlice()
		s := v.SetEmptySliceVal()
		s.AppendEmpty().SetBoolVal(true)
		s.AppendEmpty().SetBytesVal(pcommon.NewImmutableByteSlice([]byte("A")))
		return v
	}(),
	func() pcommon.Value {
		v := pcommon.NewValueSlice()
		s := v.SetEmptySliceVal()
		s.AppendEmpty().SetStringVal("")
		s.AppendEmpty().SetStringVal("")
		s.AppendEmpty().SetStringVal("")
		return v
	}(),
}

type ascendingValues []pcommon.Value

func (a ascendingValues) test(t *testing.T) {

	// Test each value matches itself
	for i := 0; i < len(a); i++ {
		require.Zero(t, compareValues(a[i], a[i]), "expected '%s' == '%s'", a[i].AsString(), a[i].AsString())
	}

	// Test each combo
	for i := 0; i+1 < len(a); i++ {
		for j := i + 1; j < len(a); j++ {
			require.Negative(t, compareValues(a[i], a[j]), "expected '%s' < '%s'", a[i].AsString(), a[j].AsString())
			require.Positive(t, compareValues(a[j], a[i]), "expected '%s' > '%s'", a[i].AsString(), a[j].AsString())
		}
	}
}
