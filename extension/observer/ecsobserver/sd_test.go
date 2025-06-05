// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ecsobserver

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
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
	fetcher := newTestTaskFetcher(t, c, c, func(options *taskFetcherOptions) {
		options.Cluster = cfg.ClusterName
		options.serviceNameFilter = svcNameFilter
	})
	opts := serviceDiscoveryOptions{Logger: logger, Fetcher: fetcher}

	// Create 1 task def, 2 services and 11 tasks, 8 running on ec2, first 3 runs on fargate
	nTasks := 11
	nInstances := 2
	nFargateInstances := 3
	c.SetTaskDefinitions(ecsmock.GenTaskDefinitions("d", 2, 1, func(i int, def *ecstypes.TaskDefinition) {
		if i == 0 {
			def.NetworkMode = ecstypes.NetworkModeAwsvpc
		} else {
			def.NetworkMode = ecstypes.NetworkModeBridge
		}
		def.ContainerDefinitions = []ecstypes.ContainerDefinition{
			{
				Name: aws.String("c1"),
				DockerLabels: map[string]string{
					"PROMETHEUS_PORT": "2112",
					"MY_JOB_NAME":     "PROM_JOB_1",
					"MY_METRICS_PATH": "/new/metrics",
				},
				PortMappings: []ecstypes.PortMapping{
					{
						ContainerPort: aws.Int32(2112),
						HostPort:      aws.Int32(2113), // doesn't matter for matcher test
					},
				},
			},
		}
	}))
	c.SetTasks(ecsmock.GenTasks("t", nTasks, func(i int, task *ecstypes.Task) {
		if i < nFargateInstances {
			task.TaskDefinitionArn = aws.String("d0:1")
			task.LaunchType = ecstypes.LaunchTypeFargate
			task.StartedBy = aws.String("deploy0")
			task.Attachments = []ecstypes.Attachment{
				{
					Type: aws.String("ElasticNetworkInterface"),
					Details: []ecstypes.KeyValuePair{
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
			task.LaunchType = ecstypes.LaunchTypeEc2
			task.ContainerInstanceArn = aws.String(fmt.Sprintf("ci%d", ins))
			task.StartedBy = aws.String("deploy1")
			task.Containers = []ecstypes.Container{
				{
					Name: aws.String("c1"),
					NetworkBindings: []ecstypes.NetworkBinding{
						{
							ContainerPort: aws.Int32(2112),
							HostPort:      aws.Int32(2114 + int32(i)),
						},
					},
				},
			}
		}
	}))
	// Setting container instance and ec2 is same as previous sub test
	c.SetContainerInstances(ecsmock.GenContainerInstances("ci", nInstances, func(i int, ci *ecstypes.ContainerInstance) {
		ci.Ec2InstanceId = aws.String(fmt.Sprintf("i-%d", i))
	}))
	c.SetEc2Instances(ecsmock.GenEc2Instances("i-", nInstances, func(i int, ins *ec2types.Instance) {
		ins.PublicIpAddress = aws.String(fmt.Sprintf("192.168.1.%d", i))
		ins.PrivateIpAddress = aws.String(fmt.Sprintf("172.168.2.%d", i))
		ins.SubnetId = aws.String(fmt.Sprintf("subnet-%d", i))
		ins.VpcId = aws.String(fmt.Sprintf("vpc-%d", i))
		ins.Tags = []ec2types.Tag{
			{
				Key:   aws.String("aws:cloudformation:instance"),
				Value: aws.String(fmt.Sprintf("cfni%d", i)),
			},
		}
	}))
	// Service
	c.SetServices(ecsmock.GenServices("s", 2, func(i int, s *ecstypes.Service) {
		if i == 0 {
			s.LaunchType = ecstypes.LaunchTypeEc2
			s.Deployments = []ecstypes.Deployment{
				{
					Status: aws.String("ACTIVE"),
					Id:     aws.String("deploy0"),
				},
			}
		} else {
			s.LaunchType = ecstypes.LaunchTypeFargate
			s.Deployments = []ecstypes.Deployment{
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
		fetcher2 := newTestTaskFetcher(t, c, c, func(options *taskFetcherOptions) {
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

func newTestTaskFetcher(t *testing.T, ecsClient ecsClient, ec2Client ec2Client, opts ...func(options *taskFetcherOptions)) *taskFetcher {
	opt := taskFetcherOptions{
		Logger:      zap.NewExample(),
		Cluster:     "not used",
		Region:      "not used",
		ecsOverride: ecsClient,
		ec2Override: ec2Client,
		serviceNameFilter: func(_ string) bool {
			return true
		},
	}
	for _, m := range opts {
		m(&opt)
	}
	f, err := newTaskFetcher(context.Background(), opt)
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
