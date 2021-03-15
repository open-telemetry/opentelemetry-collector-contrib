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

package helper

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"
)

// Duration is the representation of a length of time
type Duration struct {
	time.Duration
}

// NewDuration creates a new duration from a time
func NewDuration(t time.Duration) Duration {
	return Duration{
		Duration: t,
	}
}

// Raw will return the raw duration, without modification
func (d *Duration) Raw() time.Duration {
	return d.Duration
}

// MarshalJSON will marshal the duration as a json string
func (d Duration) MarshalJSON() ([]byte, error) {
	return []byte(`"` + d.Duration.String() + `"`), nil
}

// UnmarshalJSON will unmarshal json as a duration
func (d *Duration) UnmarshalJSON(raw []byte) error {
	var v interface{}
	err := json.Unmarshal(raw, &v)
	if err != nil {
		return err
	}
	d.Duration, err = durationFromInterface(v)
	return err
}

// MarshalYAML will marshal the duration as a yaml string
func (d Duration) MarshalYAML() (interface{}, error) {
	return d.Duration.String(), nil
}

// UnmarshalYAML will unmarshal yaml as a duration
func (d *Duration) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var v interface{}
	err := unmarshal(&v)
	if err != nil {
		return err
	}
	d.Duration, err = durationFromInterface(v)
	if d.Duration < 0 {
		d.Duration *= -1
	}
	return err
}

func durationFromInterface(val interface{}) (time.Duration, error) {
	switch value := val.(type) {
	case float64:
		return time.Duration(value * float64(time.Second)), nil
	case int:
		return time.Duration(value) * time.Second, nil
	case string:

		if _, err := strconv.Atoi(value); err == nil {
			value += "s" // int value with no unit
		}
		if _, err := strconv.ParseFloat(value, 64); err == nil {
			value += "s" // float value with no unit
		}
		d, err := time.ParseDuration(value)
		return d, err
	default:
		return 0, fmt.Errorf("cannot unmarshal value of type %T into a duration", val)
	}
}
