// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ecsobserver

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
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
		assert.Empty(t, res)
	})

	emptyTask := &taskAnnotated{
		Task: types.Task{TaskDefinitionArn: aws.String("arn:that:never:matches")},
		Definition: &types.TaskDefinition{
			TaskDefinitionArn: aws.String("arn:that:never:matches"),
			ContainerDefinitions: []types.ContainerDefinition{
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
				Task: types.Task{
					TaskDefinitionArn: aws.String("arn:alike:nginx-latest"),
				},
				Service: &types.Service{ServiceName: aws.String("nginx-service")},
				Definition: &types.TaskDefinition{
					TaskDefinitionArn: aws.String("arn:alike:nginx-latest"),
					ContainerDefinitions: []types.ContainerDefinition{
						{
							Name: aws.String("port-2112"),
							PortMappings: []types.PortMapping{
								{
									ContainerPort: aws.Int32(2112),
									HostPort:      aws.Int32(2113), // doesn't matter for matcher test
								},
							},
						},
						{
							Name: aws.String("port-2114"),
							PortMappings: []types.PortMapping{
								{
									ContainerPort: aws.Int32(2113 + 1), // a different port
									HostPort:      aws.Int32(2113),     // doesn't matter for matcher test
								},
							},
						},
					},
				},
			},
			{
				Task: types.Task{
					TaskDefinitionArn: aws.String("not used"),
				},
				Definition: &types.TaskDefinition{
					ContainerDefinitions: []types.ContainerDefinition{
						{
							Name: aws.String("port-label"),
							DockerLabels: map[string]string{
								portLabelWithInvalidValue: "not a numeric string",
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
