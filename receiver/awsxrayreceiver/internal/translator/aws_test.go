// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package translator

import (
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	conventions "go.opentelemetry.io/collector/semconv/v1.18.0"

	awsxray "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/xray"
)

func TestAddAWSToResource(t *testing.T) {
	testCases := map[string]struct {
		input *awsxray.AWSData
		want  map[string]any
	}{
		"WithNil": {
			want: map[string]any{
				conventions.AttributeCloudProvider: "unknown",
			},
		},
		"WithCloudWatchLogs": {
			input: &awsxray.AWSData{
				CWLogs: []awsxray.LogGroupMetadata{
					{
						LogGroup: aws.String("<log-group-1>"),
						Arn:      aws.String("arn:aws:logs:<region>:<account>:log-group:<log-group-1>:*"),
					},
					{
						LogGroup: aws.String("<log-group-2>"),
						Arn:      aws.String("arn:aws:logs:<region>:<account>:log-group:<log-group-2>:*"),
					},
				},
			},
			want: map[string]any{
				conventions.AttributeCloudProvider: conventions.AttributeCloudProviderAWS,
				conventions.AttributeAWSLogGroupARNs: []any{
					"arn:aws:logs:<region>:<account>:log-group:<log-group-1>:*",
					"arn:aws:logs:<region>:<account>:log-group:<log-group-2>:*",
				},
				conventions.AttributeAWSLogGroupNames: []any{"<log-group-1>", "<log-group-2>"},
			},
		},
		"WithEC2": {
			input: &awsxray.AWSData{
				EC2: &awsxray.EC2Metadata{
					InstanceID:       aws.String("<instance-id>"),
					AvailabilityZone: aws.String("<ec2-az>"),
					InstanceSize:     aws.String("<instance-size>"),
					AmiID:            aws.String("<ami>"),
				},
			},
			want: map[string]any{
				conventions.AttributeCloudProvider:         conventions.AttributeCloudProviderAWS,
				conventions.AttributeCloudAvailabilityZone: "<ec2-az>",
				conventions.AttributeHostID:                "<instance-id>",
				conventions.AttributeHostType:              "<instance-size>",
				conventions.AttributeHostImageID:           "<ami>",
			},
		},
		"WithECS": {
			input: &awsxray.AWSData{
				ECS: &awsxray.ECSMetadata{
					ContainerName:    aws.String("<container-name>"),
					ContainerID:      aws.String("<ecs-container-id>"),
					AvailabilityZone: aws.String("<ecs-az>"),
				},
			},
			want: map[string]any{
				conventions.AttributeCloudProvider:         conventions.AttributeCloudProviderAWS,
				conventions.AttributeCloudAvailabilityZone: "<ecs-az>",
				conventions.AttributeContainerName:         "<container-name>",
				conventions.AttributeContainerID:           "<ecs-container-id>",
			},
		},
		"WithEKS": {
			input: &awsxray.AWSData{
				EKS: &awsxray.EKSMetadata{
					ClusterName: aws.String("<cluster-name>"),
					Pod:         aws.String("<pod>"),
					ContainerID: aws.String("<eks-container-id>"),
				},
			},
			want: map[string]any{
				conventions.AttributeCloudProvider:  conventions.AttributeCloudProviderAWS,
				conventions.AttributeK8SPodName:     "<pod>",
				conventions.AttributeK8SClusterName: "<cluster-name>",
				conventions.AttributeContainerID:    "<eks-container-id>",
			},
		},
		"WithBeanstalk": {
			input: &awsxray.AWSData{
				Beanstalk: &awsxray.BeanstalkMetadata{
					Environment:  aws.String("<environment>"),
					VersionLabel: aws.String("<version-label>"),
					DeploymentID: aws.Int64(1),
				},
			},
			want: map[string]any{
				conventions.AttributeCloudProvider:     conventions.AttributeCloudProviderAWS,
				conventions.AttributeServiceNamespace:  "<environment>",
				conventions.AttributeServiceInstanceID: "1",
				conventions.AttributeServiceVersion:    "<version-label>",
			},
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			attrs := pcommon.NewMap()
			addAWSToResource(testCase.input, attrs)
			assert.Equal(t, testCase.want, attrs.AsRaw())
		})
	}
}
