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

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/stretchr/testify/assert"
)

func TestDockerLabelMatcher_Match(t *testing.T) {
	t.Run("skip empty config", func(t *testing.T) {
		res := newMatcherAndMatch(t, &DockerLabelConfig{}, nil)
		assert.Len(t, res.Tasks, 0)
	})

	t.Run("metrics_ports not supported", func(t *testing.T) {
		cfg := DockerLabelConfig{
			CommonExporterConfig: CommonExporterConfig{
				MetricsPorts: []int{404}, // they should be ignored
			},
		}
		assert.Error(t, cfg.Init())
	})

	portLabel := "MY_PROMETHEUS_PORT"
	jobLabel := "MY_PROMETHEUS_JOB"
	tasks := []*Task{
		{
			Definition: &ecs.TaskDefinition{
				ContainerDefinitions: []*ecs.ContainerDefinition{
					{
						DockerLabels: map[string]*string{
							portLabel: aws.String("2112"),
							jobLabel:  aws.String("PROM_JOB_1"),
						},
					},
					{
						DockerLabels: map[string]*string{
							"not" + portLabel: aws.String("bar"),
						},
					},
				},
			},
		},
	}

	t.Run("port label", func(t *testing.T) {
		cfg := DockerLabelConfig{
			PortLabel:    portLabel,
			JobNameLabel: jobLabel,
		}
		res := newMatcherAndMatch(t, &cfg, tasks)
		assert.Equal(t, &MatchResult{
			Tasks: []int{0},
			Containers: []MatchedContainer{
				{
					TaskIndex:      0,
					ContainerIndex: 0,
					Targets: []MatchedTarget{
						{
							MatcherType: MatcherTypeDockerLabel,
							Port:        2112,
							Job:         "PROM_JOB_1",
						},
					},
				},
			},
		}, res)
	})

	t.Run("override job label", func(t *testing.T) {
		cfg := DockerLabelConfig{
			PortLabel:    portLabel,
			JobNameLabel: jobLabel,
			CommonExporterConfig: CommonExporterConfig{
				JobName: "override docker label",
			},
		}
		res := newMatcherAndMatch(t, &cfg, tasks)
		assert.Equal(t, &MatchResult{
			Tasks: []int{0},
			Containers: []MatchedContainer{
				{
					TaskIndex:      0,
					ContainerIndex: 0,
					Targets: []MatchedTarget{
						{
							MatcherType: MatcherTypeDockerLabel,
							Port:        2112,
							Job:         "override docker label",
						},
					},
				},
			},
		}, res)
	})
}
