// Copyright  OpenTelemetry Authors
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

package ecsobserver

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMatchedContainer_MergeTargets(t *testing.T) {
	t.Run("add new targets", func(t *testing.T) {
		m := MatchedContainer{
			Targets: []MatchedTarget{
				{
					Port:        1234,
					MetricsPath: "/m1",
				},
				{
					Port:        1235,
					MetricsPath: "/m2",
				},
			},
		}
		newTargets := []MatchedTarget{
			{
				Port:        1234,
				MetricsPath: "/not-m1", // different path
			},
			{
				Port:        1235, // different port
				MetricsPath: "/m1",
			},
		}
		m.MergeTargets(newTargets)
		assert.Len(t, m.Targets, 4)
		assert.Equal(t, m.Targets[3].MetricsPath, "/m1") // order is append
	})

	t.Run("respect existing targets", func(t *testing.T) {
		m := MatchedContainer{
			Targets: []MatchedTarget{
				{
					MatcherType: MatcherTypeService,
					Port:        1234,
					MetricsPath: "/m1",
				},
				{
					Port:        1235,
					MetricsPath: "/m2",
				},
			},
		}
		newTargets := []MatchedTarget{
			{
				MatcherType: MatcherTypeDockerLabel, // different matcher
				Port:        1234,
				MetricsPath: "/m1",
			},
			{
				Port:        1235, // different port
				MetricsPath: "/m1",
			},
		}
		m.MergeTargets(newTargets)
		assert.Len(t, m.Targets, 3)
		assert.Equal(t, MatcherTypeService, m.Targets[0].MatcherType)
	})
}
