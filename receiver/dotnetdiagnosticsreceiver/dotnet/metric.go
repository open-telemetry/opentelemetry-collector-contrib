// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package dotnet

// Metric is a map containing all of the keys and values for a Metric extracted
// from the event parser. Most metrics will have some subset of the keys defined
// below, but some metrics/messages will have keys that deviate from this list.
type Metric map[string]interface{}

const (
	mapKeyCount                = "Count"
	mapKeyCounterType          = "CounterType"
	mapKeyDisplayName          = "DisplayName"
	mapKeyDisplayRateTimeScale = "DisplayRateTimeScale"
	mapKeyDisplayUnits         = "DisplayUnits"
	mapKeyIncrement            = "Increment"
	mapKeyIntervalSec          = "IntervalSec"
	mapKeyMax                  = "Max"
	mapKeyMean                 = "Mean"
	mapKeyMin                  = "Min"
	mapKeyName                 = "Name"
	mapKeySeries               = "Series"
	mapKeyStandardDeviation    = "StandardDeviation"
)

func (m Metric) Count() int32 {
	return m[mapKeyCount].(int32)
}

func (m Metric) CounterType() string {
	return m[mapKeyCounterType].(string)
}

func (m Metric) DisplayName() string {
	return m[mapKeyDisplayName].(string)
}

func (m Metric) DisplayRateTimeScale() string {
	return m[mapKeyDisplayRateTimeScale].(string)
}

func (m Metric) DisplayUnits() string {
	return m[mapKeyDisplayUnits].(string)
}

func (m Metric) Increment() float64 {
	return m[mapKeyIncrement].(float64)
}

func (m Metric) IntervalSec() float32 {
	return m[mapKeyIntervalSec].(float32)
}

func (m Metric) Max() float64 {
	return m[mapKeyMax].(float64)
}

func (m Metric) Mean() float64 {
	return m[mapKeyMean].(float64)
}

func (m Metric) Min() float64 {
	return m[mapKeyMin].(float64)
}

func (m Metric) Name() string {
	return m[mapKeyName].(string)
}

func (m Metric) Series() string {
	return m[mapKeySeries].(string)
}

func (m Metric) StandardDeviation() float64 {
	return m[mapKeyStandardDeviation].(float64)
}
