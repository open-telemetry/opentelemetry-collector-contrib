// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package host

import (
	"context"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"

	ci "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/containerinsight"
)

type mockEC2TagsClient func(ctx context.Context, input *ec2.DescribeTagsInput, optFns ...func(options *ec2.Options)) (*ec2.DescribeTagsOutput, error)

func (m mockEC2TagsClient) DescribeTags(ctx context.Context, input *ec2.DescribeTagsInput, optFns ...func(options *ec2.Options)) (*ec2.DescribeTagsOutput, error) {
	return m(ctx, input, optFns...)
}

func TestEC2TagsForEKS(t *testing.T) {
	tests := []struct {
		name   string
		client func(t *testing.T) ec2TagsClient
	}{
		{
			name: "EKS",
			client: func(t *testing.T) ec2TagsClient {
				return mockEC2TagsClient(func(_ context.Context, _ *ec2.DescribeTagsInput, _ ...func(*ec2.Options)) (*ec2.DescribeTagsOutput, error) {
					t.Helper()
					return &ec2.DescribeTagsOutput{
						Tags: []ec2types.TagDescription{
							{
								Key:   aws.String(clusterNameTagKeyPrefix + "cluster-name"),
								Value: aws.String("owned"),
							},
							{
								Key:   aws.String(autoScalingGroupNameTag),
								Value: aws.String("asg"),
							},
						},
					}, nil
				})
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			et := ec2Tags{
				containerOrchestrator: ci.EKS,
				client:                test.client(t),
				instanceID:            "instanceId",
				refreshInterval:       time.Millisecond,
				logger:                zap.NewNop(),
			}
			et.refresh(context.Background())
			assert.Equal(t, "cluster-name", et.getClusterName())
			assert.Equal(t, "asg", et.getAutoScalingGroupName())
		})
	}
}

func TestEC2TagsForECS(t *testing.T) {
	tests := []struct {
		name   string
		client func(t *testing.T) ec2TagsClient
	}{
		{
			name: "ECS",
			client: func(t *testing.T) ec2TagsClient {
				return mockEC2TagsClient(func(_ context.Context, _ *ec2.DescribeTagsInput, _ ...func(*ec2.Options)) (*ec2.DescribeTagsOutput, error) {
					t.Helper()
					return &ec2.DescribeTagsOutput{
						Tags: []ec2types.TagDescription{
							{
								Key:   aws.String(autoScalingGroupNameTag),
								Value: aws.String("asg"),
							},
						},
					}, nil
				})
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			et := ec2Tags{
				containerOrchestrator: ci.ECS,
				client:                test.client(t),
				instanceID:            "instanceId",
				refreshInterval:       time.Millisecond,
				logger:                zap.NewNop(),
			}
			et.refresh(context.Background())
			assert.Equal(t, "asg", et.getAutoScalingGroupName())
		})
	}
}
