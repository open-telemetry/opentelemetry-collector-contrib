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
	"bytes"
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/ecsobserver/internal/ecsmock"
)

func TestNewDiscovery(t *testing.T) {
	logger := zap.NewExample()
	outputFile := "testdata/ut_targets.actual.yaml"
	cfg := Config{
		ClusterName:     "ut-cluster-1",
		ClusterRegion:   "us-test-2",
		RefreshInterval: 10 * time.Millisecond,
		ResultFile:      outputFile,
		JobLabelName:    defaultJobLabelName,
		DockerLabels: []DockerLabelConfig{
			{
				PortLabel:        "PROMETHEUS_PORT",
				JobNameLabel:     "MY_JOB_NAME",
				MetricsPathLabel: "MY_METRICS_PATH",
			},
		},
		Services: []ServiceConfig{
			{
				NamePattern: "s1",
				CommonExporterConfig: CommonExporterConfig{
					MetricsPorts: []int{2112},
				},
			},
		},
	}
	svcNameFilter, err := serviceConfigsToFilter(cfg.Services)
	assert.True(t, svcNameFilter("s1"))
	require.NoError(t, err)
	c := ecsmock.NewClusterWithName(cfg.ClusterName)
	fetcher := newTestTaskFetcher(t, c, func(options *taskFetcherOptions) {
		options.Cluster = cfg.ClusterName
		options.serviceNameFilter = svcNameFilter
	})
	opts := serviceDiscoveryOptions{Logger: logger, Fetcher: fetcher}

	// Create 1 task def, 2 services and 11 tasks, 8 running on ec2, first 3 runs on fargate
	nTasks := 11
	nInstances := 2
	nFargateInstances := 3
	c.SetTaskDefinitions(ecsmock.GenTaskDefinitions("d", 2, 1, func(i int, def *ecs.TaskDefinition) {
		if i == 0 {
			def.NetworkMode = aws.String(ecs.NetworkModeAwsvpc)
		} else {
			def.NetworkMode = aws.String(ecs.NetworkModeBridge)
		}
		def.ContainerDefinitions = []*ecs.ContainerDefinition{
			{
				Name: aws.String("c1"),
				DockerLabels: map[string]*string{
					"PROMETHEUS_PORT": aws.String("2112"),
					"MY_JOB_NAME":     aws.String("PROM_JOB_1"),
					"MY_METRICS_PATH": aws.String("/new/metrics"),
				},
				PortMappings: []*ecs.PortMapping{
					{
						ContainerPort: aws.Int64(2112),
						HostPort:      aws.Int64(2113), // doesn't matter for matcher test
					},
				},
			},
		}
	}))
	c.SetTasks(ecsmock.GenTasks("t", nTasks, func(i int, task *ecs.Task) {
		if i < nFargateInstances {
			task.TaskDefinitionArn = aws.String("d0:1")
			task.LaunchType = aws.String(ecs.LaunchTypeFargate)
			task.StartedBy = aws.String("deploy0")
			task.Attachments = []*ecs.Attachment{
				{
					Type: aws.String("ElasticNetworkInterface"),
					Details: []*ecs.KeyValuePair{
						{
							Name:  aws.String("privateIPv4Address"),
							Value: aws.String(fmt.Sprintf("172.168.1.%d", i)),
						},
					},
				},
			}
			// Pretend this fargate task does not have private ip to trigger print non critical error.
			if i == (nFargateInstances - 1) {
				task.Attachments = nil
			}
		} else {
			ins := i % nInstances
			task.TaskDefinitionArn = aws.String("d1:1")
			task.LaunchType = aws.String(ecs.LaunchTypeEc2)
			task.ContainerInstanceArn = aws.String(fmt.Sprintf("ci%d", ins))
			task.StartedBy = aws.String("deploy1")
			task.Containers = []*ecs.Container{
				{
					Name: aws.String("c1"),
					NetworkBindings: []*ecs.NetworkBinding{
						{
							ContainerPort: aws.Int64(2112),
							HostPort:      aws.Int64(2114 + int64(i)),
						},
					},
				},
			}
		}
	}))
	// Setting container instance and ec2 is same as previous sub test
	c.SetContainerInstances(ecsmock.GenContainerInstances("ci", nInstances, func(i int, ci *ecs.ContainerInstance) {
		ci.Ec2InstanceId = aws.String(fmt.Sprintf("i-%d", i))
	}))
	c.SetEc2Instances(ecsmock.GenEc2Instances("i-", nInstances, func(i int, ins *ec2.Instance) {
		ins.PrivateIpAddress = aws.String(fmt.Sprintf("172.168.2.%d", i))
		ins.Tags = []*ec2.Tag{
			{
				Key:   aws.String("aws:cloudformation:instance"),
				Value: aws.String(fmt.Sprintf("cfni%d", i)),
			},
		}
	}))
	// Service
	c.SetServices(ecsmock.GenServices("s", 2, func(i int, s *ecs.Service) {
		if i == 0 {
			s.LaunchType = aws.String(ecs.LaunchTypeEc2)
			s.Deployments = []*ecs.Deployment{
				{
					Status: aws.String("ACTIVE"),
					Id:     aws.String("deploy0"),
				},
			}
		} else {
			s.LaunchType = aws.String(ecs.LaunchTypeFargate)
			s.Deployments = []*ecs.Deployment{
				{
					Status: aws.String("ACTIVE"),
					Id:     aws.String("deploy1"),
				},
			}
		}
	}))

	t.Run("success", func(t *testing.T) {
		sd, err := newDiscovery(cfg, opts)
		require.NoError(t, err)

		ctx, cancel := context.WithTimeout(context.Background(), cfg.RefreshInterval*2)
		defer cancel()
		err = sd.runAndWriteFile(ctx)
		require.NoError(t, err)

		assert.FileExists(t, outputFile)
		expectedFile := "testdata/ut_targets.expected.yaml"
		// workaround for windows git checkout autocrlf
		// https://circleci.com/blog/circleci-config-teardown-how-we-write-our-circleci-config-at-circleci/#main:~:text=Line%20endings
		expectedContent := bytes.ReplaceAll(mustReadFile(t, expectedFile), []byte("\r\n"), []byte("\n"))
		assert.Equal(t, string(expectedContent), string(mustReadFile(t, outputFile)))
	})

	t.Run("fail to write file", func(t *testing.T) {
		cfg2 := cfg
		cfg2.ResultFile = "testdata/folder/does/not/exists/ut_targets.yaml"
		sd, err := newDiscovery(cfg2, opts)
		require.NoError(t, err)
		require.Error(t, sd.runAndWriteFile(context.TODO()))
	})

	t.Run("critical error in discovery", func(t *testing.T) {
		cfg2 := cfg
		cfg2.ClusterName += "not_right_anymore"
		fetcher2 := newTestTaskFetcher(t, c, func(options *taskFetcherOptions) {
			options.Cluster = cfg2.ClusterName
			options.serviceNameFilter = svcNameFilter
		})
		opts2 := serviceDiscoveryOptions{Logger: logger, Fetcher: fetcher2}
		sd, err := newDiscovery(cfg2, opts2)
		require.NoError(t, err)
		require.Error(t, sd.runAndWriteFile(context.TODO()))
	})

	t.Run("invalid fetcher config", func(t *testing.T) {
		cfg2 := cfg
		cfg2.ClusterName = ""
		opts2 := serviceDiscoveryOptions{Logger: logger}
		_, err := newDiscovery(cfg2, opts2)
		require.Error(t, err)
	})
}

// Util Start

func newTestTaskFilter(t *testing.T, cfg Config) *taskFilter {
	logger := zap.NewExample()
	m, err := newMatchers(cfg, matcherOptions{Logger: logger})
	require.NoError(t, err)
	f := newTaskFilter(logger, m)
	return f
}

func newTestTaskFetcher(t *testing.T, c *ecsmock.Cluster, opts ...func(options *taskFetcherOptions)) *taskFetcher {
	opt := taskFetcherOptions{
		Logger:      zap.NewExample(),
		Cluster:     "not used",
		Region:      "not used",
		ecsOverride: c,
		ec2Override: c,
		serviceNameFilter: func(name string) bool {
			return true
		},
	}
	for _, m := range opts {
		m(&opt)
	}
	f, err := newTaskFetcher(opt)
	require.NoError(t, err)
	return f
}

func newMatcher(t *testing.T, cfg matcherConfig) targetMatcher {
	m, err := cfg.newMatcher(testMatcherOptions())
	require.NoError(t, err)
	return m
}

func newMatcherAndMatch(t *testing.T, cfg matcherConfig, tasks []*taskAnnotated) *matchResult {
	m := newMatcher(t, cfg)
	res, err := matchContainers(tasks, m, 0)
	require.NoError(t, err)
	return res
}

func testMatcherOptions() matcherOptions {
	return matcherOptions{
		Logger: zap.NewExample(),
	}
}

func mustReadFile(t *testing.T, p string) []byte {
	b, err := os.ReadFile(p)
	require.NoError(t, err, p)
	return b
}

// Util End
