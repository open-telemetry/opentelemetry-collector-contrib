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
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestNew(t *testing.T) {
	t.Run("actual fecther is not implemented", func(t *testing.T) {
		_, err := NewDiscovery(ExampleConfig(), ServiceDiscoveryOptions{})
		require.Error(t, err)
	})
}

func TestServiceDiscovery_RunAndWriteFile(t *testing.T) {
	genTasks := func() []*Task {
		return []*Task{
			{
				Task: &ecs.Task{
					TaskArn:           aws.String("arn:task:t1"),
					TaskDefinitionArn: aws.String("t1"),
					Containers: []*ecs.Container{
						{
							Name: aws.String("c1-t1"),
						},
						{
							Name: aws.String("c2-t1"),
							NetworkBindings: []*ecs.NetworkBinding{
								{
									ContainerPort: aws.Int64(1008),
									HostPort:      aws.Int64(8008),
								},
							},
						},
					},
					Tags: []*ecs.Tag{
						{
							Key:   aws.String("ecs-tag-is"),
							Value: aws.String("different struct from ec2.Tag"),
						},
					},
				},
				Definition: &ecs.TaskDefinition{
					NetworkMode: aws.String(ecs.NetworkModeBridge),
					ContainerDefinitions: []*ecs.ContainerDefinition{
						{
							Name: aws.String("c1-t1"),
						},
						{
							Name: aws.String("c2-t1"),
							DockerLabels: map[string]*string{
								"PROMETHEUS_PORT": aws.String("1008"),
							},
							PortMappings: []*ecs.PortMapping{
								{
									ContainerPort: aws.Int64(1008),
								},
							},
						},
					},
				},
				Service: &ecs.Service{
					ServiceName: aws.String("s1"),
				},
				EC2: &ec2.Instance{
					PrivateIpAddress: aws.String("172.168.0.1"),
					Tags: []*ec2.Tag{
						{
							Key:   aws.String("aws:cloudformation:is:not:valid:prometheus:label"),
							Value: aws.String("but it will ber sanitized"),
						},
					},
				},
			},
			{
				Task: &ecs.Task{
					TaskArn:           aws.String("arn:task:t2"),
					TaskDefinitionArn: aws.String("t2"),
					Attachments: []*ecs.Attachment{
						{
							Type: aws.String("ElasticNetworkInterface"),
							Details: []*ecs.KeyValuePair{
								{
									Name:  aws.String("privateIPv4Address"),
									Value: aws.String("172.168.1.1"),
								},
							},
						},
					},
				},
				Definition: &ecs.TaskDefinition{
					NetworkMode: aws.String(ecs.NetworkModeAwsvpc),
					ContainerDefinitions: []*ecs.ContainerDefinition{
						{
							Name: aws.String("c1-t2"),
							DockerLabels: map[string]*string{
								"NOT_PORT": aws.String("just a value"),
							},
						},
						{
							Name: aws.String("c2-t2"),
							DockerLabels: map[string]*string{
								"PROMETHEUS_PORT": aws.String("2112"),
							},
							PortMappings: []*ecs.PortMapping{
								{
									ContainerPort: aws.Int64(2112),
									HostPort:      aws.Int64(8112),
								},
							},
						},
					},
				},
				Service: &ecs.Service{
					ServiceName: aws.String("s2"),
				},
			},
		}
	}

	outputFile := "testdata/ut_targets.actual.yaml"
	cfg := Config{
		ClusterName:     "ut-cluster-1",
		ClusterRegion:   "us-test-2",
		RefreshInterval: 100 * time.Millisecond,
		ResultFile:      outputFile,
		JobLabelName:    DefaultJobLabelName,
		DockerLabels: []DockerLabelConfig{
			{
				PortLabel: "PROMETHEUS_PORT",
			},
		},
	}
	opts := ServiceDiscoveryOptions{
		Logger: zap.NewExample(),
		FetcherOverride: newMockFetcher(func() ([]*Task, error) {
			return genTasks(), nil
		}),
	}

	t.Run("success", func(t *testing.T) {
		sd, err := NewDiscovery(cfg, opts)
		require.NoError(t, err)
		ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
		defer cancel()
		require.NoError(t, sd.RunAndWriteFile(ctx))
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
		sd, err := NewDiscovery(cfg2, opts)
		require.NoError(t, err)
		require.Error(t, sd.RunAndWriteFile(context.TODO()))
	})

	t.Run("invalid discovery", func(t *testing.T) {
		optsNoFetch := ServiceDiscoveryOptions{
			Logger: zap.NewExample(),
			FetcherOverride: newMockFetcher(func() ([]*Task, error) {
				return nil, fmt.Errorf("discovery should fail")
			}),
		}
		sd, err := NewDiscovery(cfg, optsNoFetch)
		require.NoError(t, err)
		require.Error(t, sd.RunAndWriteFile(context.TODO()))
	})

}

// Util Start

func mustReadFile(t *testing.T, p string) []byte {
	b, err := ioutil.ReadFile(p)
	require.NoError(t, err, p)
	return b
}

func newMatcher(t *testing.T, cfg MatcherConfig) Matcher {
	require.NoError(t, cfg.Init())
	m, err := cfg.NewMatcher(testMatcherOptions())
	require.NoError(t, err)
	return m
}

func newMatcherAndMatch(t *testing.T, cfg MatcherConfig, tasks []*Task) *MatchResult {
	m := newMatcher(t, cfg)
	res, err := matchContainers(tasks, m, 0)
	require.NoError(t, err)
	return res
}

func testMatcherOptions() MatcherOptions {
	return MatcherOptions{
		Logger: zap.NewExample(),
	}
}

// Util End
