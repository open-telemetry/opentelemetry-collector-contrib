// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
package ec2

import (
	"testing"

	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

const (
	testIP         = "ip-12-34-56-78.us-west-2.compute.internal"
	testDomu       = "domu-12-34-56-78.us-west-2.compute.internal"
	testEC2        = "ec2amaz-12-34-56-78.us-west-2.compute.internal"
	customHost     = "custom-hostname"
	testInstanceID = "i-0123456789"
)

func TestDefaultHostname(t *testing.T) {
	assert.True(t, isDefaultHostname(testIP))
	assert.True(t, isDefaultHostname(testDomu))
	assert.True(t, isDefaultHostname(testEC2))
	assert.False(t, isDefaultHostname(customHost))
}

func TestGetHostname(t *testing.T) {
	logger := zap.NewNop()

	hostInfo := &HostInfo{
		InstanceID:  testInstanceID,
		EC2Hostname: testIP,
	}
	assert.Equal(t, testInstanceID, hostInfo.GetHostname(logger))

	hostInfo = &HostInfo{
		InstanceID:  testInstanceID,
		EC2Hostname: customHost,
	}
	assert.Equal(t, customHost, hostInfo.GetHostname(logger))
}

func strp(s string) *string {
	return &s
}

func TestClusterNameFromEC2Tags(t *testing.T) {
	tests := []struct {
		name        string
		ec2Tags     *ec2.DescribeTagsOutput
		clusterName string
		err         string
	}{
		{
			name: "missing cluster name tag",
			ec2Tags: &ec2.DescribeTagsOutput{
				NextToken: strp("NextToken"),
				Tags: []*ec2.TagDescription{
					{Key: strp("some key"), Value: strp("some value")},
				},
			},
			err: "no tag found with prefix \"kubernetes.io/cluster/\"",
		},
		{
			name: "cluster name tag only has the prefix",
			ec2Tags: &ec2.DescribeTagsOutput{
				NextToken: strp("NextToken"),
				Tags: []*ec2.TagDescription{
					{Key: strp("some key"), Value: strp("some value")},
					{Key: strp("kubernetes.io/cluster/"), Value: strp("some value")},
				},
			},
			err: "missing cluster name in \"kubernetes.io/cluster/\" tag",
		},
		{
			name: "cluster name is available",
			ec2Tags: &ec2.DescribeTagsOutput{
				NextToken: strp("NextToken"),
				Tags: []*ec2.TagDescription{
					{Key: strp("some key"), Value: strp("some value")},
					{Key: strp("kubernetes.io/cluster/myclustername"), Value: strp("some value")},
				},
			},
			clusterName: "myclustername",
		},
	}

	for _, testInstance := range tests {
		t.Run(testInstance.name, func(t *testing.T) {
			clusterName, err := clusterNameFromTags(testInstance.ec2Tags)
			if err != nil || testInstance.err != "" {
				assert.EqualError(t, err, testInstance.err)
			} else {
				assert.Equal(t, testInstance.clusterName, clusterName)
			}
		})
	}
}
