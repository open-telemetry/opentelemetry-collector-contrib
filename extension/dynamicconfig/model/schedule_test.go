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
	"bytes"
	"testing"

	pb "github.com/open-telemetry/opentelemetry-collector-contrib/extension/dynamicconfig/proto/experimental/metrics/configservice"
)

func TestScheduleProto(t *testing.T) {
	schedule := Schedule{
		InclusionPatterns: []Pattern{{Equals: "one"}, {StartsWith: "two"}},
		ExclusionPatterns: []Pattern{{StartsWith: "three"}, {Equals: "four"}},
		Period:            "MIN_5",
	}

	p, err := schedule.Proto()
	if err != nil || len(p.InclusionPatterns) != 2 ||
		len(p.ExclusionPatterns) != 2 ||
		p.PeriodSec != 300 {
		t.Errorf("improper conversion to proto")
	}

	if p.InclusionPatterns[0].Match.(*pb.MetricConfigResponse_Schedule_Pattern_Equals).Equals != "one" ||
		p.InclusionPatterns[1].Match.(*pb.MetricConfigResponse_Schedule_Pattern_StartsWith).StartsWith != "two" ||
		p.ExclusionPatterns[0].Match.(*pb.MetricConfigResponse_Schedule_Pattern_StartsWith).StartsWith != "three" ||
		p.ExclusionPatterns[1].Match.(*pb.MetricConfigResponse_Schedule_Pattern_Equals).Equals != "four" {

		t.Errorf("proto patterns incorrect: expected one, two, three, four, got: %v", schedule)
	}
}

func TestScheduleBadPeriod(t *testing.T) {
	schedule := Schedule{
		InclusionPatterns: []Pattern{{Equals: "one"}, {StartsWith: "two"}},
		ExclusionPatterns: []Pattern{{StartsWith: "three"}, {Equals: "four"}},
		Period:            "MIN_3",
	}

	if _, err := schedule.Proto(); err == nil {
		t.Errorf("expected schedule with Period=%v to be invalid", schedule.Period)
	}
}

func TestScheduleBadPattern(t *testing.T) {
	schedule := Schedule{
		InclusionPatterns: []Pattern{{Equals: "one", StartsWith: "two"}},
		Period:            "MIN_5",
	}

	if _, err := schedule.Proto(); err == nil {
		t.Errorf("expected schedule with InclusionPattern=%v to be invalid", schedule.InclusionPatterns)
	}

	schedule = Schedule{
		ExclusionPatterns: []Pattern{{Equals: "one", StartsWith: "two"}},
		Period:            "MIN_5",
	}

	if _, err := schedule.Proto(); err == nil {
		t.Errorf("expected schedule with ExclusionPatterns=%v to be invalid", schedule.ExclusionPatterns)
	}
}

func TestScheduleHash(t *testing.T) {
	configA := Schedule{
		InclusionPatterns: []Pattern{
			{Equals: "woot"},
			{StartsWith: "yay"},
		},
	}

	configB := Schedule{
		InclusionPatterns: []Pattern{
			{StartsWith: "yay"},
			{Equals: "woot"},
		},
	}

	configC := Schedule{
		ExclusionPatterns: []Pattern{
			{Equals: "woot"},
			{StartsWith: "yay"},
		},
	}

	if !bytes.Equal(configA.Hash(), configB.Hash()) {
		t.Errorf("identical configs with different hashes")
	}

	if bytes.Equal(configA.Hash(), configC.Hash()) {
		t.Errorf("different configs with identical hashes")
	}
}
