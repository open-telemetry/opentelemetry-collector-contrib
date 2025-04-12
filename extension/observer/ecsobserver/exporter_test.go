// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ecsobserver

import (
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/multierr"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestTaskExporter(t *testing.T) {
	exp := newTaskExporter(zap.NewExample(), "ecs-cluster-1")

	t.Run("invalid ip", func(t *testing.T) {
		_, err := exp.exportTask(&taskAnnotated{
			Task:       &ecs.Task{},
			Definition: &ecs.TaskDefinition{},
		})
		assert.Error(t, err)
		v := &errPrivateIPNotFound{}
		assert.ErrorAs(t, err, &v)
	})

	awsVpcTask := &ecs.Task{
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
		Containers: []*ecs.Container{
			{
				Name: aws.String("c1"),
			},
		},
	}
	awsVpcTaskDef := &ecs.TaskDefinition{
		ContainerDefinitions: []*ecs.ContainerDefinition{
			{
				PortMappings: []*ecs.PortMapping{
					{
						ContainerPort: aws.Int64(2112),
						HostPort:      aws.Int64(2113),
					},
				},
			},
		},
		NetworkMode: aws.String(ecs.NetworkModeAwsvpc),
	}
	t.Run("single target", func(t *testing.T) {
		task := &taskAnnotated{
			Task:       awsVpcTask,
			Definition: awsVpcTaskDef,
			Matched: []matchedContainer{
				{
					Targets: []matchedTarget{
						{
							MatcherType: matcherTypeDockerLabel,
							Port:        2112,
							Job:         "PROM_JOB_1",
						},
					},
				},
			},
		}

		targets, err := exp.exportTask(task)
		require.NoError(t, err)
		assert.Len(t, targets, 1)
		assert.Equal(t, "172.168.1.1:2113", targets[0].Address)
	})

	t.Run("multiple target in one container", func(t *testing.T) {
		task := &taskAnnotated{
			Task:       awsVpcTask,
			Definition: awsVpcTaskDef,
			Service:    &ecs.Service{ServiceName: aws.String("svc-1")},
			Matched: []matchedContainer{
				{
					Targets: []matchedTarget{
						{
							MatcherType: matcherTypeDockerLabel,
							Port:        2112,
							Job:         "PROM_JOB_1",
						},
						{
							Port: 404, // invalid in the middle, but shouldn't stop the export
						},
						{
							MatcherType: matcherTypeService,
							Port:        2112,
							Job:         "PROM_JOB_Service",
							MetricsPath: "/service_metrics",
						},
					},
				},
			},
		}

		targets, err := exp.exportTask(task)
		require.Error(t, err)
		merr := multierr.Errors(err)
		require.Len(t, merr, 1)
		v := &errMappedPortNotFound{}
		assert.ErrorAs(t, merr[0], &v)
		assert.Len(t, targets, 2)
	})

	t.Run("ec2", func(t *testing.T) {
		task := &taskAnnotated{
			Task: &ecs.Task{
				TaskArn:           aws.String("arn:task:t2"),
				TaskDefinitionArn: aws.String("t2"),
				Containers: []*ecs.Container{
					{
						Name: aws.String("c1"),
						NetworkBindings: []*ecs.NetworkBinding{
							{
								ContainerPort: aws.Int64(2112),
								HostPort:      aws.Int64(2114),
							},
						},
					},
				},
			},
			Definition: &ecs.TaskDefinition{
				ContainerDefinitions: []*ecs.ContainerDefinition{
					{
						Name: aws.String("c1"),
						PortMappings: []*ecs.PortMapping{
							{
								ContainerPort: aws.Int64(2112),
							},
						},
					},
				},
				NetworkMode: aws.String(ecs.NetworkModeBridge),
			},
			EC2: &ec2.Instance{PrivateIpAddress: aws.String("172.168.1.2")},
			Matched: []matchedContainer{
				{
					Targets: []matchedTarget{
						{
							MatcherType: matcherTypeDockerLabel,
							Port:        2112,
							Job:         "PROM_JOB_1",
						},
					},
				},
			},
		}

		targets, err := exp.exportTask(task)
		require.NoError(t, err)
		assert.Len(t, targets, 1)
		assert.Equal(t, "172.168.1.2:2114", targets[0].Address)
	})

	validMatched := []matchedContainer{
		{
			Targets: []matchedTarget{
				{
					MatcherType: matcherTypeDockerLabel,
					Port:        2112,
					Job:         "PROM_JOB_1",
				},
			},
		},
	}
	invalidMatched := []matchedContainer{
		{
			Targets: []matchedTarget{
				{
					MatcherType: matcherTypeDockerLabel,
					Port:        404,
					Job:         "PROM_JOB_1",
				},
				{
					MatcherType: matcherTypeDockerLabel,
					Port:        405,
					Job:         "PROM_JOB_1",
				},
			},
		},
	}
	validTask := &taskAnnotated{
		Task:       awsVpcTask,
		Definition: awsVpcTaskDef,
		Matched:    validMatched,
	}
	invalidTargetTask := &taskAnnotated{
		Task:       awsVpcTask,
		Definition: awsVpcTaskDef,
		Matched:    invalidMatched,
	}
	invalidIPTask := &taskAnnotated{
		Task:       &ecs.Task{TaskArn: aws.String("invalid task's invalid arn")},
		Definition: &ecs.TaskDefinition{},
	}
	t.Run("all valid tasks", func(t *testing.T) {
		// Just make sure the for loop is right, done care about the content, they are tested in previous cases
		targets, err := exp.exportTasks([]*taskAnnotated{validTask, validTask})
		assert.NoError(t, err)
		assert.Len(t, targets, 2)
	})

	t.Run("invalid tasks", func(t *testing.T) {
		targets, err := exp.exportTasks([]*taskAnnotated{
			validTask,
			invalidTargetTask, validTask, invalidTargetTask, invalidTargetTask,
			invalidIPTask,
		})
		require.Error(t, err)
		assert.Len(t, targets, 2)
		merr := multierr.Errors(err)
		// each invalid task has two invalid match, we have 3 invalid tasks
		// multierr flatten multierr when appending
		assert.Len(t, merr, 2*3+1)

		zCore, logs := observer.New(zap.ErrorLevel)
		printErrors(zap.New(zCore), err)
		assert.Len(t, logs.All(), len(merr))

		taskScope := logs.FilterField(zap.String("ErrScope", "taskAnnotated"))
		assert.Equal(t, 1, taskScope.Len())
		targetScope := logs.FilterField(zap.String("ErrScope", "Target"))
		assert.Equal(t, 2*3, targetScope.Len())

		printErrors(zap.NewExample(), err) // inspect during development
	})
}
