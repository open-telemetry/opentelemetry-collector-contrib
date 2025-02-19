// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package host

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/awstesting/mock"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"

	ci "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/containerinsight"
)

type mockEC2TagsClient struct {
	count                 int
	tokenString           string
	clusterKey            string
	clusterValue          string
	asgKey                string
	asgValue              string
	containerOrchestrator string
}

func (m *mockEC2TagsClient) DescribeTagsWithContext(_ context.Context, _ *ec2.DescribeTagsInput,
	_ ...request.Option,
) (*ec2.DescribeTagsOutput, error) {
	m.count++
	if m.count == 1 {
		return &ec2.DescribeTagsOutput{}, errors.New("error")
	}

	if m.count == 2 {
		return &ec2.DescribeTagsOutput{
			NextToken: &m.tokenString,
			Tags: []*ec2.TagDescription{
				{
					Key:   &m.asgKey,
					Value: &m.asgValue,
				},
			},
		}, nil
	}

	return &ec2.DescribeTagsOutput{
		Tags: []*ec2.TagDescription{
			{
				Key:   &m.clusterKey,
				Value: &m.clusterValue,
			},
		},
	}, nil
}

func TestEC2TagsForEKS(t *testing.T) {
	ctx := context.Background()
	sess := mock.Session
	clientOption := func(e *ec2Tags) {
		e.client = &mockEC2TagsClient{
			tokenString:           "tokenString",
			clusterKey:            clusterNameTagKeyPrefix + "cluster-name",
			clusterValue:          "owned",
			asgKey:                autoScalingGroupNameTag,
			asgValue:              "asg",
			containerOrchestrator: ci.EKS,
		}
	}
	maxJitterOption := func(e *ec2Tags) {
		e.maxJitterTime = 0
	}
	isSuccessOption := func(e *ec2Tags) {
		e.isSuccess = make(chan bool)
	}
	et := newEC2Tags(ctx, sess, "instanceId", "us-west-2", ci.EKS, time.Millisecond, zap.NewNop(), clientOption,
		maxJitterOption, isSuccessOption)

	// wait for ec2 tags are fetched
	e := et.(*ec2Tags)
	<-e.isSuccess
	assert.Equal(t, "cluster-name", et.getClusterName())
	assert.Equal(t, "asg", et.getAutoScalingGroupName())
}

func TestEC2TagsForECS(t *testing.T) {
	ctx := context.Background()
	sess := mock.Session
	clientOption := func(e *ec2Tags) {
		e.client = &mockEC2TagsClient{
			tokenString:           "tokenString",
			clusterKey:            clusterNameTagKeyPrefix + "cluster-name",
			clusterValue:          "",
			asgKey:                autoScalingGroupNameTag,
			asgValue:              "asg",
			containerOrchestrator: ci.ECS,
		}
	}
	maxJitterOption := func(e *ec2Tags) {
		e.maxJitterTime = 0
	}
	isSuccessOption := func(e *ec2Tags) {
		e.isSuccess = make(chan bool)
	}
	et := newEC2Tags(ctx, sess, "instanceId", "us-west-2", ci.ECS, time.Millisecond, zap.NewNop(), clientOption,
		maxJitterOption, isSuccessOption)

	// wait for ec2 tags are fetched
	e := et.(*ec2Tags)
	<-e.isSuccess
	assert.Equal(t, "asg", et.getAutoScalingGroupName())
}
