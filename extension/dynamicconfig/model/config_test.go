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

package model

import (
	"testing"

	com "github.com/open-telemetry/opentelemetry-proto/gen/go/common/v1"
	res "github.com/open-telemetry/opentelemetry-proto/gen/go/resource/v1"
)

var config = Config{
	ConfigBlocks: []*ConfigBlock{
		{
			Resource: []string{
				"status:1",
				"running:true",
				"location:infirmary",
			},
			Schedules: []*Schedule{
				{Period: "SEC_1"},
			},
		},
		{
			Resource: []string{
				"status: 1",
			},
			Schedules: []*Schedule{
				{Period: "SEC_5"},
			},
		},
		{
			Resource: nil,
			Schedules: []*Schedule{
				{Period: "DAY_1"},
			},
		},
		{
			Resource: []string{
				"status:0",
			},
			Schedules: []*Schedule{
				{Period: "DAY_7"},
			},
		},
	},
}

func TestMatch(t *testing.T) {
	resource := &res.Resource{
		Attributes: []*com.KeyValue{
			{Key: "status", Value: &com.AnyValue{Value: &com.AnyValue_IntValue{IntValue: 1}}},
			{Key: "running", Value: &com.AnyValue{Value: &com.AnyValue_BoolValue{BoolValue: true}}},
			{Key: "location", Value: &com.AnyValue{Value: &com.AnyValue_StringValue{StringValue: "infirmary"}}},
		},
	}

	result := config.Match(resource)
	scheds := result.Schedules

	schedLen := len(scheds)
	if schedLen != 3 {
		t.Errorf("expected to have three schedules, got: %v", schedLen)
	}

	if scheds[0].Period != "SEC_1" || scheds[1].Period != "SEC_5" || scheds[2].Period != "DAY_1" {
		t.Errorf("expected periods to be SEC_1, SEC_5 and DAY_1 respectively, got schedules: %v", scheds)
	}

	if result.Resource[0] != "status:1" ||
		result.Resource[1] != "running:true" ||
		result.Resource[2] != "location:infirmary" {

		t.Errorf("result resource list incorrect: %v", result)
	}
}

func TestMatchEmptyResource(t *testing.T) {
	resource := &res.Resource{}

	result := config.Match(resource)
	scheds := result.Schedules

	schedlen := len(scheds)
	if schedlen != 1 {
		t.Errorf("expected to have one schedule, got: %v", schedlen)
	}

	if scheds[0].Period != "DAY_1" {
		t.Errorf("expected period to be DAY_1, got: %v", scheds)
	}

	if len(result.Resource) > 0 {
		t.Errorf("expected resource list to be empty, got: %v", result)
	}
}

func TestMatchNilResource(t *testing.T) {
	var resource *res.Resource = nil

	result := config.Match(resource)
	scheds := result.Schedules

	schedlen := len(scheds)
	if schedlen != 1 {
		t.Errorf("expected to have one schedule, got: %v", schedlen)
	}

	if scheds[0].Period != "DAY_1" {
		t.Errorf("expected period to be DAY_1, got: %v", scheds)
	}

	if len(result.Resource) > 0 {
		t.Errorf("expected resource list to be empty, got: %v", result)
	}
}

func TestMatchNoResource(t *testing.T) {
	resource := &res.Resource{
		Attributes: []*com.KeyValue{
			{Key: "secret", Value: &com.AnyValue{Value: &com.AnyValue_IntValue{IntValue: 69105}}},
		},
	}

	result := config.Match(resource)
	scheds := result.Schedules

	schedlen := len(scheds)
	if schedlen != 1 {
		t.Errorf("expected to have one schedule, got: %v", schedlen)
	}

	if scheds[0].Period != "DAY_1" {
		t.Errorf("expected period to be DAY_1, got: %v", scheds)
	}

	if result.Resource[0] != "secret:69105" {
		t.Errorf("result resource list incorrect: %v", result)
	}
}
