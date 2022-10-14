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
	pcommon.NewValueStr(""),
	pcommon.NewValueInt(0),
	pcommon.NewValueDouble(0),
	pcommon.NewValueBool(false),
	pcommon.NewValueMap(),
	pcommon.NewValueSlice(),
	byteSlice(""),
}

var pBools ascendingValues = []pcommon.Value{
	pcommon.NewValueBool(false),
	pcommon.NewValueBool(true),
}

var pStrings ascendingValues = []pcommon.Value{
	pcommon.NewValueStr(""),
	pcommon.NewValueStr(" "),
	pcommon.NewValueStr("A"),
	pcommon.NewValueStr("AA"),
	pcommon.NewValueStr("_"),
	pcommon.NewValueStr("a"),
	pcommon.NewValueStr("aa"),
	pcommon.NewValueStr("b"),
	pcommon.NewValueStr("long"),
	pcommon.NewValueStr("z"),
}

var pBytes ascendingValues = []pcommon.Value{
	byteSlice(""),
	byteSlice(" "),
	byteSlice("A"),
	byteSlice("AA"),
	byteSlice("_"),
	byteSlice("a"),
	byteSlice("aa"),
	byteSlice("b"),
	byteSlice("long"),
	byteSlice("z"),
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
		m := v.SetEmptyMap()
		m.PutStr("a", "")
		return v
	}(),
	func() pcommon.Value {
		v := pcommon.NewValueMap()
		m := v.SetEmptyMap()
		m.PutInt("a", 123)
		return v
	}(),
	func() pcommon.Value {
		v := pcommon.NewValueMap()
		m := v.SetEmptyMap()
		m.PutInt("b", 100)
		return v
	}(),
	func() pcommon.Value {
		v := pcommon.NewValueMap()
		m := v.SetEmptyMap()
		m.PutStr("c", "")
		return v
	}(),
	func() pcommon.Value {
		v := pcommon.NewValueMap()
		m := v.SetEmptyMap()
		m.PutStr("a", "")
		m.PutStr("b", "")
		return v
	}(),
	func() pcommon.Value {
		v := pcommon.NewValueMap()
		m := v.SetEmptyMap()
		m.PutStr("a", "")
		m.PutEmptyMap("m")
		return v
	}(),
	func() pcommon.Value {
		v := pcommon.NewValueMap()
		m := v.SetEmptyMap()
		m.PutStr("a", "")
		mv := m.PutEmptyMap("m")
		mv.PutInt("i", 0)
		return v
	}(),
	func() pcommon.Value {
		v := pcommon.NewValueMap()
		m := v.SetEmptyMap()
		m.PutStr("a", "")
		vs := m.PutEmptySlice("s")
		d := vs.AppendEmpty()
		d.SetDouble(100)
		return v
	}(),
}

// ascending lengths -> types -> values
var pSlices ascendingValues = []pcommon.Value{
	pcommon.NewValueSlice(),
	func() pcommon.Value {
		v := pcommon.NewValueSlice()
		s := v.SetEmptySlice()
		s.AppendEmpty().SetStr("")
		return v
	}(),
	func() pcommon.Value {
		v := pcommon.NewValueSlice()
		s := v.SetEmptySlice()
		s.AppendEmpty().SetInt(100)
		return v
	}(),
	func() pcommon.Value {
		v := pcommon.NewValueSlice()
		s := v.SetEmptySlice()
		s.AppendEmpty().SetDouble(100)
		return v
	}(),
	func() pcommon.Value {
		v := pcommon.NewValueSlice()
		s := v.SetEmptySlice()
		s.AppendEmpty().SetBool(true)
		return v
	}(),
	func() pcommon.Value {
		v := pcommon.NewValueSlice()
		s := v.SetEmptySlice()
		s.AppendEmpty().SetEmptyMap()
		return v
	}(),
	func() pcommon.Value {
		v := pcommon.NewValueSlice()
		s := v.SetEmptySlice()
		m := s.AppendEmpty().SetEmptyMap()
		m.PutStr("a", "")
		return v
	}(),
	func() pcommon.Value {
		v := pcommon.NewValueSlice()
		s := v.SetEmptySlice()
		m := s.AppendEmpty().SetEmptyMap()
		m.PutStr("b", "")
		return v
	}(),
	func() pcommon.Value {
		v := pcommon.NewValueSlice()
		s := v.SetEmptySlice()
		s.AppendEmpty().SetEmptySlice()
		return v
	}(),
	func() pcommon.Value {
		v := pcommon.NewValueSlice()
		s := v.SetEmptySlice()
		ns := s.AppendEmpty().SetEmptySlice()
		ns.AppendEmpty().SetStr("")
		return v
	}(),
	func() pcommon.Value {
		v := pcommon.NewValueSlice()
		s := v.SetEmptySlice()
		s.AppendEmpty().SetEmptyBytes().Append([]byte("A")...)
		return v
	}(),
	func() pcommon.Value {
		v := pcommon.NewValueSlice()
		s := v.SetEmptySlice()
		s.AppendEmpty().SetStr("")
		s.AppendEmpty().SetStr("a")
		return v
	}(),
	func() pcommon.Value {
		v := pcommon.NewValueSlice()
		s := v.SetEmptySlice()
		s.AppendEmpty().SetStr("")
		s.AppendEmpty().SetBool(false)
		return v
	}(),
	func() pcommon.Value {
		v := pcommon.NewValueSlice()
		s := v.SetEmptySlice()
		s.AppendEmpty().SetInt(0)
		s.AppendEmpty().SetBool(true)
		return v
	}(),
	func() pcommon.Value {
		v := pcommon.NewValueSlice()
		s := v.SetEmptySlice()
		s.AppendEmpty().SetInt(100)
		s.AppendEmpty().SetInt(0)
		return v
	}(),
	func() pcommon.Value {
		v := pcommon.NewValueSlice()
		s := v.SetEmptySlice()
		s.AppendEmpty().SetInt(100)
		s.AppendEmpty().SetDouble(0)
		return v
	}(),
	func() pcommon.Value {
		v := pcommon.NewValueSlice()
		s := v.SetEmptySlice()
		s.AppendEmpty().SetBool(false)
		s.AppendEmpty().SetBool(true)
		return v
	}(),
	func() pcommon.Value {
		v := pcommon.NewValueSlice()
		s := v.SetEmptySlice()
		s.AppendEmpty().SetBool(true)
		s.AppendEmpty().SetBool(false)
		return v
	}(),
	func() pcommon.Value {
		v := pcommon.NewValueSlice()
		s := v.SetEmptySlice()
		s.AppendEmpty().SetBool(true)
		s.AppendEmpty().SetEmptyBytes().Append([]byte("A")...)
		return v
	}(),
	func() pcommon.Value {
		v := pcommon.NewValueSlice()
		s := v.SetEmptySlice()
		s.AppendEmpty().SetStr("")
		s.AppendEmpty().SetStr("")
		s.AppendEmpty().SetStr("")
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

func byteSlice(str string) pcommon.Value {
	val := pcommon.NewValueBytes()
	val.SetEmptyBytes().Append([]byte(str)...)
	return val
}
