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

package faroreceiver

import (
	"fmt"
	"sort"

	om "github.com/wk8/go-ordered-map"
)

// KeyVal is an ordered map of string to interface
type KeyVal = om.OrderedMap

// NewKeyVal creates new empty KeyVal
func NewKeyVal() *KeyVal {
	return om.New()
}

// KeyValFromMap will instantiate KeyVal from a map[string]string
func KeyValFromMap(m map[string]string) *KeyVal {
	kv := NewKeyVal()
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		KeyValAdd(kv, k, m[k])
	}
	return kv
}

// MergeKeyVal will merge source in target
func MergeKeyVal(target *KeyVal, source *KeyVal) {
	for el := source.Oldest(); el != nil; el = el.Next() {
		target.Set(el.Key, el.Value)
	}
}

// MergeKeyValWithPrefix will merge source in target, adding a prefix to each key being merged in
func MergeKeyValWithPrefix(target *KeyVal, source *KeyVal, prefix string) {
	for el := source.Oldest(); el != nil; el = el.Next() {
		target.Set(fmt.Sprintf("%s%s", prefix, el.Key), el.Value)
	}
}

// KeyValAdd adds a key + value string pair to kv
func KeyValAdd(kv *KeyVal, key string, value string) {
	if len(value) > 0 {
		kv.Set(key, value)
	}
}

// KeyValToInterfaceSlice converts KeyVal to []interface{}, typically used for logging
func KeyValToInterfaceSlice(kv *KeyVal) []interface{} {
	slice := make([]interface{}, kv.Len()*2)
	idx := 0
	for el := kv.Oldest(); el != nil; el = el.Next() {
		slice[idx] = el.Key
		idx++
		slice[idx] = el.Value
		idx++
	}
	return slice
}

// KeyValToInterfaceMap converts KeyVal to map[string]interface
func KeyValToInterfaceMap(kv *KeyVal) map[string]interface{} {
	retv := make(map[string]interface{})
	for el := kv.Oldest(); el != nil; el = el.Next() {
		retv[fmt.Sprint(el.Key)] = el.Value
	}
	return retv
}
