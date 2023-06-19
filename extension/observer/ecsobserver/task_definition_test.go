// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ecsobserver

import (
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTaskDefinitionMatcher(t *testing.T) {
	t.Run("must set arn pattern", func(t *testing.T) {
		var cfg TaskDefinitionConfig
		require.Error(t, cfg.validate())
	})

	t.Run("invalid regex", func(t *testing.T) {
		invalidRegex := "*" //  missing argument to repetition operator: `*`
		cfg := TaskDefinitionConfig{ArnPattern: invalidRegex}
		require.Error(t, cfg.validate())

		cfg = TaskDefinitionConfig{ArnPattern: "arn:is:valid:regexp", ContainerNamePattern: invalidRegex}
		require.Error(t, cfg.validate())
	})

	t.Run("invalid export config", func(t *testing.T) {
		cfg := TaskDefinitionConfig{ArnPattern: "a", CommonExporterConfig: CommonExporterConfig{
			MetricsPorts: []int{8080, 8080},
		}}
		require.Error(t, cfg.validate())
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
	genTasks := func() []*taskAnnotated {
		return []*taskAnnotated{
			{
				Task: &ecs.Task{
					TaskDefinitionArn: aws.String("arn:alike:nginx-latest"),
				},
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
			emptyTask,
		}
	}

	t.Run("arn only", func(t *testing.T) {
		cfg := TaskDefinitionConfig{
			ArnPattern: "nginx",
			CommonExporterConfig: CommonExporterConfig{
				JobName:      "CONFIG_PROM_JOB",
				MetricsPorts: []int{2112},
			},
		}
		res := newMatcherAndMatch(t, &cfg, genTasks())
		assert.Equal(t, &matchResult{
			Tasks: []int{0},
			Containers: []matchedContainer{
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
			},
		}, res)
	})

	t.Run("container name", func(t *testing.T) {
		cfg := TaskDefinitionConfig{
			ArnPattern:           "nginx",
			ContainerNamePattern: ".*-2114",
			CommonExporterConfig: CommonExporterConfig{
				JobName:      "CONFIG_PROM_JOB",
				MetricsPorts: []int{2112, 2114},
			},
		}
		res := newMatcherAndMatch(t, &cfg, genTasks())
		assert.Equal(t, &matchResult{
			Tasks: []int{0},
			Containers: []matchedContainer{
				{
					TaskIndex:      0,
					ContainerIndex: 1,
					Targets: []matchedTarget{
						{
							MatcherType: matcherTypeTaskDefinition,
							Port:        2114,
							Job:         "CONFIG_PROM_JOB",
						},
					},
				},
			},
		}, res)
	})
}
