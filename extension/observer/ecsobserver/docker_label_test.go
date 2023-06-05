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

func TestDockerLabelMatcher_Match(t *testing.T) {
	t.Run("must set port label", func(t *testing.T) {
		var cfg DockerLabelConfig
		require.Error(t, cfg.validate())
	})

	t.Run("metrics_ports not supported", func(t *testing.T) {
		cfg := DockerLabelConfig{
			PortLabel: "foo",
			CommonExporterConfig: CommonExporterConfig{
				MetricsPorts: []int{404},
			},
		}
		assert.Error(t, cfg.validate())
	})

	t.Run("invalid export config", func(t *testing.T) {
		cfg := DockerLabelConfig{PortLabel: "foo", CommonExporterConfig: CommonExporterConfig{
			MetricsPorts: []int{8080, 8080},
		}}
		require.Error(t, cfg.validate())
	})

	t.Run("valid", func(t *testing.T) {
		cfg := DockerLabelConfig{
			PortLabel: "PORT_PROM",
		}
		assert.NoError(t, cfg.validate())
	})

	portLabel := "MY_PROMETHEUS_PORT"
	portLabelWithInvalidValue := "MY_PROMETHEUS_PORT_IS_INVALID"
	portLabelWithoutMapping := "MY_PROMETHEUS_PORT_IS_NOT_THERE"
	jobLabel := "MY_PROMETHEUS_JOB"
	metricsPathLabel := "MY_METRICS_PATH"

	genTasks := func() []*taskAnnotated {
		return []*taskAnnotated{
			{
				Definition: &ecs.TaskDefinition{
					ContainerDefinitions: []*ecs.ContainerDefinition{
						{
							DockerLabels: map[string]*string{
								portLabel:        aws.String("2112"),
								jobLabel:         aws.String("PROM_JOB_1"),
								metricsPathLabel: aws.String("/new/metrics"),
							},
							PortMappings: []*ecs.PortMapping{
								{
									ContainerPort: aws.Int64(2112),
									HostPort:      aws.Int64(2113), // doesn't matter for matcher test
								},
							},
						},
						{
							DockerLabels: map[string]*string{
								"not" + portLabel: aws.String("bar"),
							},
							// no port mapping at all
						},
						{
							// port value in label does not match container port.
							// most likely a misconfiguration or the labels are attached by tools.
							DockerLabels: map[string]*string{
								portLabelWithoutMapping: aws.String("2113"),
							},
							PortMappings: []*ecs.PortMapping{
								{
									ContainerPort: aws.Int64(2113 + 1), // a different port from label value
								},
							},
						},
					},
				},
			},
			{
				Definition: &ecs.TaskDefinition{
					ContainerDefinitions: []*ecs.ContainerDefinition{
						{
							DockerLabels: map[string]*string{
								portLabelWithInvalidValue: aws.String("not a port number"),
							},
						},
					},
				},
			},
		}
	}

	t.Run("port label", func(t *testing.T) {
		cfg := DockerLabelConfig{
			PortLabel:    portLabel,
			JobNameLabel: jobLabel,
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
							MatcherType: matcherTypeDockerLabel,
							Port:        2112,
							Job:         "PROM_JOB_1",
						},
					},
				},
			},
		}, res)
	})

	t.Run("port mapping not found", func(t *testing.T) {
		cfg := DockerLabelConfig{
			PortLabel: portLabelWithoutMapping,
		}
		m := newMatcher(t, &cfg)
		// Direct match has error
		_, err := m.matchTargets(genTasks()[0], genTasks()[0].Definition.ContainerDefinitions[2])
		require.Error(t, err)

		// errNotMatched is ignored
		_, err = matchContainers(genTasks(), m, 0)
		require.NoError(t, err)
	})

	t.Run("invalid port label value", func(t *testing.T) {
		cfg := DockerLabelConfig{
			PortLabel: portLabelWithInvalidValue,
		}
		m := newMatcher(t, &cfg)
		res, err := matchContainers(genTasks(), m, 0)
		require.Error(t, err)
		errs := multierr.Errors(err)
		assert.Len(t, errs, 1)
		assert.NotNil(t, res, "return non nil res even if there are some errors, don't drop all task because one invalid task")
	})

	t.Run("metrics path", func(t *testing.T) {
		cfg := DockerLabelConfig{
			PortLabel:        portLabel,
			MetricsPathLabel: metricsPathLabel,
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
							MatcherType: matcherTypeDockerLabel,
							Port:        2112,
							MetricsPath: "/new/metrics",
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
		res := newMatcherAndMatch(t, &cfg, genTasks())
		assert.Equal(t, &matchResult{
			Tasks: []int{0},
			Containers: []matchedContainer{
				{
					TaskIndex:      0,
					ContainerIndex: 0,
					Targets: []matchedTarget{
						{
							MatcherType: matcherTypeDockerLabel,
							Port:        2112,
							Job:         "override docker label",
						},
					},
				},
			},
		}, res)
	})
}
