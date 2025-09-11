// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ecsobserver

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTask_Tags(t *testing.T) {
	t.Run("ec2", func(t *testing.T) {
		task := taskAnnotated{}
		assert.Equal(t, map[string]string(nil), task.EC2Tags())
		task.EC2 = &ec2types.Instance{
			Tags: []ec2types.Tag{
				{
					Key:   aws.String("k"),
					Value: aws.String("v"),
				},
			},
		}
		assert.Equal(t, map[string]string{"k": "v"}, task.EC2Tags())
	})

	t.Run("task", func(t *testing.T) {
		task := taskAnnotated{Task: ecstypes.Task{}}
		assert.Equal(t, map[string]string(nil), task.TaskTags())
		task.Task.Tags = []ecstypes.Tag{
			{
				Key:   aws.String("k"),
				Value: aws.String("v"),
			},
		}
		assert.Equal(t, map[string]string{"k": "v"}, task.TaskTags())
	})

	t.Run("container", func(t *testing.T) {
		task := taskAnnotated{Definition: &ecstypes.TaskDefinition{ContainerDefinitions: []ecstypes.ContainerDefinition{{}}}}
		assert.Equal(t, map[string]string(nil), task.ContainerLabels(0))
		task.Definition.ContainerDefinitions[0].DockerLabels = map[string]string{
			"k": "v",
		}
		assert.Equal(t, map[string]string{"k": "v"}, task.ContainerLabels(0))
	})
}

func TestTask_PrivateIP(t *testing.T) {
	t.Run("awsvpc", func(t *testing.T) {
		task := taskAnnotated{
			Task: ecstypes.Task{
				TaskArn:           aws.String("arn:task:t2"),
				TaskDefinitionArn: aws.String("t2"),
				Attachments: []ecstypes.Attachment{
					{
						Type: aws.String("ElasticNetworkInterface"),
						Details: []ecstypes.KeyValuePair{
							{
								Name:  aws.String("privateIPv4Address"),
								Value: aws.String("172.168.1.1"),
							},
						},
					},
				},
			},
			Definition: &ecstypes.TaskDefinition{NetworkMode: ecstypes.NetworkModeAwsvpc},
		}
		ip, err := task.PrivateIP()
		require.NoError(t, err)
		assert.Equal(t, "172.168.1.1", ip)
	})

	t.Run("not found", func(t *testing.T) {
		task := taskAnnotated{
			Task:       ecstypes.Task{TaskArn: aws.String("arn:task:1")},
			Definition: &ecstypes.TaskDefinition{},
		}
		modes := []ecstypes.NetworkMode{"", ecstypes.NetworkModeBridge, ecstypes.NetworkModeHost, ecstypes.NetworkModeAwsvpc, ecstypes.NetworkModeNone, "not even a network mode"}
		for _, mode := range modes {
			task.Definition.NetworkMode = mode
			_, err := task.PrivateIP()
			assert.Error(t, err)
			errPINF := &errPrivateIPNotFound{}
			require.ErrorAs(t, err, &errPINF)
			assert.Equal(t, mode, errPINF.NetworkMode)
		}
	})
}

func TestTask_MappedPort(t *testing.T) {
	ec2BridgeTask := ecstypes.Task{
		TaskArn: aws.String("arn:task:1"),
		Containers: []ecstypes.Container{
			{
				Name: aws.String("c1"),
				NetworkBindings: []ecstypes.NetworkBinding{
					{
						ContainerPort: aws.Int32(2112),
						HostPort:      aws.Int32(2345),
					},
				},
			},
		},
	}
	// Network mode is optional for ecs and it default to ec2 bridge
	t.Run("empty is ec2 bridge", func(t *testing.T) {
		task := taskAnnotated{
			Task:       ec2BridgeTask,
			Definition: &ecstypes.TaskDefinition{NetworkMode: ""},
			EC2:        &ec2types.Instance{PrivateIpAddress: aws.String("172.168.1.1")},
		}
		ip, err := task.PrivateIP()
		require.NoError(t, err)
		assert.Equal(t, "172.168.1.1", ip)
		p, err := task.MappedPort(ecstypes.ContainerDefinition{Name: aws.String("c1")}, 2112)
		require.NoError(t, err)
		assert.Equal(t, int32(2345), p)
	})

	t.Run("ec2 bridge", func(t *testing.T) {
		task := taskAnnotated{
			Task:       ec2BridgeTask,
			Definition: &ecstypes.TaskDefinition{NetworkMode: ecstypes.NetworkModeBridge},
			EC2:        &ec2types.Instance{PrivateIpAddress: aws.String("172.168.1.1")},
		}
		p, err := task.MappedPort(ecstypes.ContainerDefinition{Name: aws.String("c1")}, 2112)
		require.NoError(t, err)
		assert.Equal(t, int32(2345), p)
	})

	vpcTaskDef := &ecstypes.TaskDefinition{
		NetworkMode: ecstypes.NetworkModeAwsvpc,
		ContainerDefinitions: []ecstypes.ContainerDefinition{
			{
				Name: aws.String("c1"),
				PortMappings: []ecstypes.PortMapping{
					{
						ContainerPort: aws.Int32(2112),
						HostPort:      aws.Int32(2345),
					},
				},
			},
		},
	}
	t.Run("awsvpc", func(t *testing.T) {
		task := taskAnnotated{
			Task:       ecstypes.Task{TaskArn: aws.String("arn:task:1")},
			Definition: vpcTaskDef,
		}
		p, err := task.MappedPort(vpcTaskDef.ContainerDefinitions[0], 2112)
		require.NoError(t, err)
		assert.Equal(t, int32(2345), p)
	})

	t.Run("host", func(t *testing.T) {
		def := vpcTaskDef
		def.NetworkMode = ecstypes.NetworkModeHost
		task := taskAnnotated{
			Task:       ecstypes.Task{TaskArn: aws.String("arn:task:1")},
			Definition: def,
		}
		p, err := task.MappedPort(def.ContainerDefinitions[0], 2112)
		require.NoError(t, err)
		assert.Equal(t, int32(2345), p)
	})

	t.Run("not found", func(t *testing.T) {
		task := taskAnnotated{
			Task:       ecstypes.Task{TaskArn: aws.String("arn:task:1")},
			Definition: &ecstypes.TaskDefinition{},
		}
		modes := []ecstypes.NetworkMode{"", ecstypes.NetworkModeBridge, ecstypes.NetworkModeHost, ecstypes.NetworkModeAwsvpc, ecstypes.NetworkModeNone, "not even a network mode"}
		for _, mode := range modes {
			task.Definition.NetworkMode = mode
			_, err := task.MappedPort(ecstypes.ContainerDefinition{Name: aws.String("c11")}, 1234)
			assert.Error(t, err)
			errMPNF := &errMappedPortNotFound{}
			require.ErrorAs(t, err, &errMPNF)
			assert.Equal(t, mode, errMPNF.NetworkMode)
			assert.ErrorContains(t, err, string(mode)) // for coverage
		}
	})
}

func TestTask_AddMatchedContainer(t *testing.T) {
	t.Run("different container", func(t *testing.T) {
		task := taskAnnotated{
			Matched: []matchedContainer{
				{
					ContainerIndex: 0,
					Targets: []matchedTarget{
						{
							MatcherType: matcherTypeService,
							Port:        1,
						},
					},
				},
			},
		}

		task.AddMatchedContainer(matchedContainer{
			ContainerIndex: 1,
			Targets: []matchedTarget{
				{
					MatcherType: matcherTypeDockerLabel,
					Port:        2,
				},
			},
		})
		assert.Equal(t, []matchedContainer{
			{
				ContainerIndex: 0,
				Targets: []matchedTarget{
					{
						MatcherType: matcherTypeService,
						Port:        1,
					},
				},
			},
			{
				ContainerIndex: 1,
				Targets: []matchedTarget{
					{
						MatcherType: matcherTypeDockerLabel,
						Port:        2,
					},
				},
			},
		}, task.Matched)
	})

	t.Run("same container different metrics path", func(t *testing.T) {
		task := taskAnnotated{
			Matched: []matchedContainer{
				{
					ContainerIndex: 0,
					Targets: []matchedTarget{
						{
							MatcherType: matcherTypeService,
							Port:        1,
						},
					},
				},
			},
		}
		task.AddMatchedContainer(matchedContainer{
			ContainerIndex: 0,
			Targets: []matchedTarget{
				{
					MatcherType: matcherTypeTaskDefinition,
					Port:        1,
					MetricsPath: "/metrics2",
				},
			},
		})
		assert.Equal(t, []matchedContainer{
			{
				ContainerIndex: 0,
				Targets: []matchedTarget{
					{
						MatcherType: matcherTypeService,
						Port:        1,
					},
					{
						MatcherType: matcherTypeTaskDefinition,
						Port:        1,
						MetricsPath: "/metrics2",
					},
				},
			},
		}, task.Matched)
	})

	t.Run("same container same metrics path", func(t *testing.T) {
		task := taskAnnotated{
			Matched: []matchedContainer{
				{
					ContainerIndex: 0,
					Targets: []matchedTarget{
						{
							MatcherType: matcherTypeService,
							Port:        1,
						},
					},
				},
			},
		}
		task.AddMatchedContainer(matchedContainer{
			ContainerIndex: 0,
			Targets: []matchedTarget{
				{
					MatcherType: matcherTypeTaskDefinition,
					Port:        1,
					MetricsPath: "",
				},
			},
		})
		assert.Equal(t, []matchedContainer{
			{
				ContainerIndex: 0,
				Targets: []matchedTarget{
					{
						MatcherType: matcherTypeService,
						Port:        1,
					},
				},
			},
		}, task.Matched)
	})
}
