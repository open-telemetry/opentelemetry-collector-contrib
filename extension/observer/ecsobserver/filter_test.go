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

package ecsobserver

import (
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/multierr"
)

func TestFilter(t *testing.T) {
	cfgTaskDefOnly := Config{
		TaskDefinitions: []TaskDefinitionConfig{
			{
				ArnPattern: "arn:alike:nginx-.*",
				CommonExporterConfig: CommonExporterConfig{
					JobName:      "CONFIG_PROM_JOB",
					MetricsPorts: []int{2112},
				},
			},
		},
	}
	t.Run("nil", func(t *testing.T) {
		f := newTestTaskFilter(t, cfgTaskDefOnly)
		res, err := f.filter(nil)
		require.NoError(t, err)
		assert.Nil(t, res)
	})

	emptyTask := &taskAnnotated{
		Task: &ecs.Task{TaskDefinitionArn: aws.String("arn:that:never:matches")},
		Definition: &ecs.TaskDefinition{
			TaskDefinitionArn: aws.String("arn:that:never:matches"),
			ContainerDefinitions: []*ecs.ContainerDefinition{
				{
					Name: aws.String("I got nothing, just to trigger the for loop ~~for coverage~~"),
				},
			},
		},
	}
	portLabelWithInvalidValue := "MY_PROMETHEUS_PORT_IS_INVALID"
	genTasks := func() []*taskAnnotated {
		return []*taskAnnotated{
			{
				Task: &ecs.Task{
					TaskDefinitionArn: aws.String("arn:alike:nginx-latest"),
				},
				Service: &ecs.Service{ServiceName: aws.String("nginx-service")},
				Definition: &ecs.TaskDefinition{
					TaskDefinitionArn: aws.String("arn:alike:nginx-latest"),
					ContainerDefinitions: []*ecs.ContainerDefinition{
						{
							Name: aws.String("port-2112"),
							PortMappings: []*ecs.PortMapping{
								{
									ContainerPort: aws.Int64(2112),
									HostPort:      aws.Int64(2113), // doesn't matter for matcher test
								},
							},
						},
						{
							Name: aws.String("port-2114"),
							PortMappings: []*ecs.PortMapping{
								{
									ContainerPort: aws.Int64(2113 + 1), // a different port
									HostPort:      aws.Int64(2113),     // doesn't matter for matcher test
								},
							},
						},
					},
				},
			},
			{
				Task: &ecs.Task{
					TaskDefinitionArn: aws.String("not used"),
				},
				Definition: &ecs.TaskDefinition{
					ContainerDefinitions: []*ecs.ContainerDefinition{
						{
							Name: aws.String("port-label"),
							DockerLabels: map[string]*string{
								portLabelWithInvalidValue: aws.String("not a numeric string"),
							},
						},
					},
				},
			},
			emptyTask,
		}
	}

	t.Run("task definition", func(t *testing.T) {
		f := newTestTaskFilter(t, cfgTaskDefOnly)
		res, err := f.filter(genTasks())
		require.NoError(t, err)
		assert.Len(t, res, 1)
		assert.Equal(t, []matchedContainer{
			{
				TaskIndex:      0,
				ContainerIndex: 0,
				Targets: []matchedTarget{
					{
						MatcherType: matcherTypeTaskDefinition,
						Port:        2112,
						Job:         "CONFIG_PROM_JOB",
					},
				},
			},
			{
				TaskIndex:      0,
				ContainerIndex: 1,
				Targets:        nil, // the container itself is matched, but it has no matching port
			},
		}, res[0].Matched)
	})

	cfgServiceTaskDef := Config{
		Services: []ServiceConfig{
			{
				NamePattern: "^nginx-.*$",
				CommonExporterConfig: CommonExporterConfig{
					JobName:      "CONFIG_PROM_JOB_BY_SERVICE",
					MetricsPorts: []int{2112},
				},
			},
		},
		TaskDefinitions: []TaskDefinitionConfig{
			{
				ArnPattern: "arn:alike:nginx-.*",
				CommonExporterConfig: CommonExporterConfig{
					JobName:      "CONFIG_PROM_JOB",
					MetricsPorts: []int{2112},
				},
			},
		},
	}

	t.Run("match order", func(t *testing.T) {
		f := newTestTaskFilter(t, cfgServiceTaskDef)
		res, err := f.filter(genTasks())
		require.NoError(t, err)
		assert.Len(t, res, 1)
		assert.Equal(t, []matchedContainer{
			{
				TaskIndex:      0,
				ContainerIndex: 0,
				Targets: []matchedTarget{
					{
						MatcherType: matcherTypeService,
						Port:        2112,
						Job:         "CONFIG_PROM_JOB_BY_SERVICE",
					},
				},
			},
			{
				TaskIndex:      0,
				ContainerIndex: 1,
				Targets:        nil, // the container itself is matched, but it has no matching port
			},
		}, res[0].Matched)
	})

	cfgServiceDockerLabel := Config{
		Services: []ServiceConfig{
			{
				NamePattern: "^nginx-.*$",
				CommonExporterConfig: CommonExporterConfig{
					JobName:      "CONFIG_PROM_JOB_BY_SERVICE",
					MetricsPorts: []int{2112},
				},
			},
		},
		DockerLabels: []DockerLabelConfig{
			{
				PortLabel: portLabelWithInvalidValue,
			},
		},
	}

	t.Run("invalid docker label", func(t *testing.T) {
		f := newTestTaskFilter(t, cfgServiceDockerLabel)
		res, err := f.filter(genTasks())
		require.Error(t, err)
		merr := multierr.Errors(err)
		require.Len(t, merr, 1)
		assert.Len(t, res, 1)
	})
}
