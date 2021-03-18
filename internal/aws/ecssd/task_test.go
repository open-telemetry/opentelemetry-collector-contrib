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

package ecssd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTask_AddMatchedContainer(t *testing.T) {
	task := Task{
		Matched: []MatchedContainer{
			{
				ContainerIndex: 0,
				Targets: []MatchedTarget{
					{
						MatcherType: MatcherTypeService,
						Port:        1,
					},
				},
			},
		},
	}

	// Different container
	task.AddMatchedContainer(MatchedContainer{
		ContainerIndex: 1,
		Targets: []MatchedTarget{
			{
				MatcherType: MatcherTypeDockerLabel,
				Port:        2,
			},
		},
	})
	assert.Equal(t, []MatchedContainer{
		{
			ContainerIndex: 0,
			Targets: []MatchedTarget{
				{
					MatcherType: MatcherTypeService,
					Port:        1,
				},
			},
		},
		{
			ContainerIndex: 1,
			Targets: []MatchedTarget{
				{
					MatcherType: MatcherTypeDockerLabel,
					Port:        2,
				},
			},
		},
	}, task.Matched)

	// Same container different metrics path
	task.AddMatchedContainer(MatchedContainer{
		ContainerIndex: 0,
		Targets: []MatchedTarget{
			{
				MatcherType: MatcherTypeTaskDefinition,
				Port:        1,
				MetricsPath: "/metrics2",
			},
		},
	})
	assert.Equal(t, []MatchedContainer{
		{
			ContainerIndex: 0,
			Targets: []MatchedTarget{
				{
					MatcherType: MatcherTypeService,
					Port:        1,
				},
				{
					MatcherType: MatcherTypeTaskDefinition,
					Port:        1,
					MetricsPath: "/metrics2",
				},
			},
		},
		{
			ContainerIndex: 1,
			Targets: []MatchedTarget{
				{
					MatcherType: MatcherTypeDockerLabel,
					Port:        2,
				},
			},
		},
	}, task.Matched)
}
