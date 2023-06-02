// Copyright The OpenTelemetry Authors
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

package extractors

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func AssertContainsTaggedFloat(
	t *testing.T,
	cadvisorMetric *CAdvisorMetric,
	field string,
	expectedValue float64,
	delta float64,
) {
	var actualValue float64
	fields := cadvisorMetric.GetFields()
	if val, ok := fields[field]; ok {
		if val, ok := val.(float64); ok {
			actualValue = val
			if (val >= expectedValue-delta) && (val <= expectedValue+delta) {
				// Found the point, return without failing
				return
			}
		} else {
			assert.Fail(t, fmt.Sprintf("Field \"%s\" does not have type float64", field))
		}
	}
	msg := fmt.Sprintf(
		"Could not find field \"%s\" with requested tags within %f of %f, Actual: %f",
		field, delta, expectedValue, actualValue)
	assert.Fail(t, msg)
}

func AssertContainsTaggedInt(
	t *testing.T,
	cadvisorMetric *CAdvisorMetric,
	field string,
	expectedValue int64,
) {
	var actualValue int64
	fields := cadvisorMetric.GetFields()
	if val, ok := fields[field]; ok {
		var isOK bool
		if actualValue, isOK = val.(int64); isOK {
			return
		}
	}
	msg := fmt.Sprintf(
		"Could not find field \"%s\" with requested tags with value: %v, Actual: %v",
		field, expectedValue, actualValue)
	assert.Fail(t, msg)
}

func AssertContainsTaggedUint(
	t *testing.T,
	cadvisorMetric *CAdvisorMetric,
	field string,
	expectedValue uint64,
) {
	var actualValue uint64
	fields := cadvisorMetric.GetFields()
	if val, ok := fields[field]; ok {
		var isOK bool
		if _, isOK = val.(uint64); isOK {
			return
		}
	}
	msg := fmt.Sprintf(
		"Could not find field \"%s\" with requested tags with value: %v, Actual: %v",
		field, expectedValue, actualValue)
	assert.Fail(t, msg)
}

func AssertContainsTaggedField(
	t *testing.T,
	cadvisorMetric *CAdvisorMetric,
	expectedFields map[string]interface{},
	expectedTags map[string]string,
) {

	actualFields := cadvisorMetric.GetFields()
	actualTags := cadvisorMetric.GetTags()
	if !reflect.DeepEqual(expectedTags, actualTags) {
		msg := fmt.Sprintf("No field exists for metric %v\n", *cadvisorMetric)
		msg += fmt.Sprintf("expected: %v\n", expectedTags)
		msg += fmt.Sprintf("actual: %v\n", actualTags)
		assert.Fail(t, msg)
	}
	if len(actualFields) > 0 {
		assert.Equal(t, expectedFields, actualFields)
		return
	}
	msg := fmt.Sprintf("No field exists for metric %v", *cadvisorMetric)
	assert.Fail(t, msg)
}
