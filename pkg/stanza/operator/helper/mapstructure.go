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

package helper // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"

import (
	"encoding/json"
	"reflect"

	"github.com/mitchellh/mapstructure"
)

// make mapstructure use struct UnmarshalJSON to decode
func JSONUnmarshalerHook() mapstructure.DecodeHookFunc {
	return func(from reflect.Value, to reflect.Value) (interface{}, error) {
		if to.CanAddr() {
			to = to.Addr()
		}

		// If the destination implements the unmarshaling interface
		u, ok := to.Interface().(json.Unmarshaler)
		if !ok {
			return from.Interface(), nil
		}

		// If it is nil and a pointer, create and assign the target value first
		if to.IsNil() && to.Type().Kind() == reflect.Ptr {
			to.Set(reflect.New(to.Type().Elem()))
			u = to.Interface().(json.Unmarshaler)
		}
		v := from.Interface()
		bytes, err := json.Marshal(v)
		if err != nil {
			return nil, err
		}
		if err := u.UnmarshalJSON(bytes); err != nil {
			return to.Interface(), err
		}
		return to.Interface(), nil
	}
}

func UnmarshalMapstructure(input interface{}, result interface{}) error {
	dc := &mapstructure.DecoderConfig{Result: result, DecodeHook: JSONUnmarshalerHook()}
	ms, err := mapstructure.NewDecoder(dc)
	if err != nil {
		return err
	}
	return ms.Decode(input)
}
