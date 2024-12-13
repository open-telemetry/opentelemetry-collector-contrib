// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package stores

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"

	ci "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/containerinsight"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/cadvisor/testutils"
)

// get error when $HOST_NAME is not set
// no error when it is set

var logger = zap.NewNop()

func TestNewLocalNodeDecorator(t *testing.T) {
	// don't set HostName environment variable, expect error in eks
	d, err := NewLocalNodeDecorator(logger, "eks", nil, "")
	assert.Nil(t, d)
	assert.Error(t, err)

	// don't expect error in ecs
	d, err = NewLocalNodeDecorator(logger, "ecs", nil, "")
	assert.NotNil(t, d)
	assert.NoError(t, err)
	assert.Empty(t, d.nodeName)

	d, err = NewLocalNodeDecorator(logger, "eks", nil, "test-hostname")
	assert.NotNil(t, d)
	assert.NoError(t, err)
	assert.Equal(t, "test-hostname", d.nodeName)
}

func TestEbsVolumeInfo(t *testing.T) {
	hostInfo := testutils.MockHostInfo{}
	d, err := NewLocalNodeDecorator(logger, "eks", hostInfo, "host")
	assert.NotNil(t, d)
	assert.NoError(t, err)

	m := NewCIMetric("metric-type", logger)
	result := d.Decorate(m)
	assert.False(t, result.HasTag(ci.HostEbsVolumeID))
	assert.False(t, result.HasTag(ci.EbsVolumeID))

	m = NewCIMetricWithData("metric-type", map[string]any{}, map[string]string{ci.DiskDev: "my-disk"}, logger)
	result = d.Decorate(m)
	assert.True(t, result.HasTag(ci.HostEbsVolumeID))
	assert.False(t, result.HasTag(ci.EbsVolumeID))

	var deviceName string
	for key := range hostInfo.ExtractEbsIDsUsedByKubernetes() {
		deviceName = key
		break
	}
	for _, mType := range []string{ci.TypeContainerFS, ci.TypeNodeFS, ci.TypeNodeDiskIO, ci.TypeContainerDiskIO} {
		m = NewCIMetricWithData(mType, map[string]any{}, map[string]string{ci.DiskDev: deviceName}, logger)
		result = d.Decorate(m)
		assert.True(t, result.HasTag(ci.HostEbsVolumeID))
		assert.True(t, result.HasTag(ci.EbsVolumeID))
	}
}

type mockK8sDecorator struct{}

func (d mockK8sDecorator) Decorate(m CIMetric) CIMetric {
	m.AddTag("k8s-decorated", "true")
	return m
}

func (d mockK8sDecorator) Shutdown() error {
	return nil
}

func TestExpectedTags(t *testing.T) {
	hostInfo := testutils.MockHostInfo{ClusterName: "my-cluster"}
	k8sDecorator := mockK8sDecorator{}
	ecsInfo := testutils.MockECSInfo{}

	testCases := map[string]struct {
		containerOrchestrator string
		metricType            string
		expectedTags          map[string]string
	}{
		"EksGenericMetricType": {
			containerOrchestrator: "eks",
			metricType:            "metric-type",
			expectedTags: map[string]string{
				ci.Version:                 "0",
				ci.AutoScalingGroupNameKey: "asg",           // from MockHostInfo
				ci.InstanceID:              "instance-id",   // from MockHostInfo
				ci.InstanceType:            "instance-type", // from MockHostInfo
				ci.ClusterNameKey:          "my-cluster",
				"k8s-decorated":            "true",
			},
		},
		"EksNodeMetricType": {
			containerOrchestrator: "eks",
			metricType:            ci.TypeNode,
			expectedTags: map[string]string{
				ci.Version:                 "0",
				ci.AutoScalingGroupNameKey: "asg",           // from MockHostInfo
				ci.InstanceID:              "instance-id",   // from MockHostInfo
				ci.InstanceType:            "instance-type", // from MockHostInfo
				ci.NodeNameKey:             "host",
				ci.ClusterNameKey:          "my-cluster",
				"k8s-decorated":            "true",
			},
		},
		"EcsInstanceMetricType": {
			containerOrchestrator: "ecs",
			metricType:            ci.TypeInstance,
			expectedTags: map[string]string{
				ci.Version:                 "0",
				ci.AutoScalingGroupNameKey: "asg",           // from MockHostInfo
				ci.InstanceID:              "instance-id",   // from MockHostInfo
				ci.InstanceType:            "instance-type", // from MockHostInfo
				ci.NodeNameKey:             "host",
				ci.ClusterNameKey:          "ecs-cluster", // from MockECSInfo
				ci.ContainerInstanceIDKey:  "eeee12.dsfr", // from MockECSInfo
				ci.SourcesKey:              "[\"cadvisor\",\"/proc\",\"ecsagent\",\"calculated\"]",
			},
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			d, err := NewLocalNodeDecorator(logger, testCase.containerOrchestrator, hostInfo, "host", WithK8sDecorator(k8sDecorator), WithECSInfo(&ecsInfo))
			assert.NotNil(t, d)
			assert.NoError(t, err)

			m := NewCIMetric(testCase.metricType, logger)
			testCase.expectedTags[ci.MetricType] = testCase.metricType
			result := d.Decorate(m)
			assert.Equal(t, testCase.expectedTags, result.GetTags())
		})
	}
}
