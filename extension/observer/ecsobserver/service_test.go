// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ecsobserver

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServiceMatcher(t *testing.T) {
	t.Run("must set name pattern", func(t *testing.T) {
		var cfg ServiceConfig
		require.Error(t, cfg.validate())
	})

	t.Run("invalid regex", func(t *testing.T) {
		invalidRegex := "*" //  missing argument to repetition operator: `*`
		cfg := ServiceConfig{NamePattern: invalidRegex}
		require.Error(t, cfg.validate())

		_, err := serviceConfigsToFilter([]ServiceConfig{cfg})
		require.Error(t, err)

		cfg = ServiceConfig{NamePattern: "valid", ContainerNamePattern: invalidRegex}
		require.Error(t, cfg.validate())
	})

	t.Run("invalid export config", func(t *testing.T) {
		cfg := ServiceConfig{NamePattern: "valid", CommonExporterConfig: CommonExporterConfig{
			MetricsPorts: []int{8080, 8080},
		}}
		require.Error(t, cfg.validate())
	})

	emptyDef := &types.TaskDefinition{
		ContainerDefinitions: []types.ContainerDefinition{
			{
				Name: aws.String("I got nothing, just to trigger the for loop ~~for coverage~~"),
			},
		},
	}
	genTasks := func() []*taskAnnotated {
		return []*taskAnnotated{
			{
				Service: &types.Service{ServiceName: aws.String("nginx-service")},
				Definition: &types.TaskDefinition{
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
				Service:    &types.Service{ServiceName: aws.String("memcached-service")},
				Definition: emptyDef,
			},
			{
				// No service
				Definition: emptyDef,
			},
		}
	}

	t.Run("service name only", func(t *testing.T) {
		cfg := ServiceConfig{
			NamePattern: "nginx",
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
							MatcherType: matcherTypeService,
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
		cfg := ServiceConfig{
			NamePattern:          "nginx",
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
							MatcherType: matcherTypeService,
							Port:        2114,
							Job:         "CONFIG_PROM_JOB",
						},
					},
				},
			},
		}, res)
	})
}

func TestServiceNameFilter(t *testing.T) {
	t.Run("match nothing when empty", func(t *testing.T) {
		f, err := serviceConfigsToFilter(nil)
		require.NoError(t, err)
		require.False(t, f("should not match"))
	})

	t.Run("invalid regex", func(t *testing.T) {
		invalidRegex := "*" //  missing argument to repetition operator: `*`
		cfg := ServiceConfig{NamePattern: invalidRegex}
		_, err := serviceConfigsToFilter([]ServiceConfig{cfg})
		require.Error(t, err)
	})
}
