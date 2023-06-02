// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build linux
// +build linux

package cadvisor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/cadvisor/extractors"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/cadvisor/testutils"
)

func TestIsContainerInContainer(t *testing.T) {
	cases := []struct {
		name string
		path string
		dind bool
	}{
		{
			"guaranteed",
			"/kubepods.slice/kubepods-podc8f7bb69_65f2_4b61_ae5a_9b19ac47a239.slice/docker-523b624a86a2a74c2bedf586d8448c86887ef7858a8dec037d6559e5ad3fccb5.scope",
			false,
		},
		{
			"guaranteed-dind",
			"/kubepods.slice/kubepods-burstable-podc9adcee4_c874_4dad_8bc8_accdbd67ac3a.slice/docker-e58cfbc8b67f6e1af458efdd31cb2a8abdbf9f95db64f4c852b701285a09d40e.scope/docker/fb651068cfbd4bf3d45fb092ec9451f8d1a36b3753687bbaa0a9920617eae5b9",
			true,
		},
		{
			"burstable",
			"/kubepods.slice/kubepods-besteffort.slice/kubepods-besteffort-podab0e310c_0bdb_48e8_ac87_81a701514645.slice/docker-caa8a5e51cd6610f8f0110b491e8187d23488b9635acccf0355a7975fd3ff158.scope",
			false,
		},
		{
			"burstable-dind",
			"/kubepods.slice/kubepods-burstable.slice/kubepods-burstable-podc9adcee4_c874_4dad_8bc8_accdbd67ac3a.slice/docker-e58cfbc8b67f6e1af458efdd31cb2a8abdbf9f95db64f4c852b701285a09d40e.scope/docker/fb651068cfbd4bf3d45fb092ec9451f8d1a36b3753687bbaa0a9920617eae5b9",
			true,
		},
	}
	for _, c := range cases {
		assert.Equal(t, c.dind, isContainerInContainer(c.path), c.name)
	}
}

func TestProcessContainers(t *testing.T) {
	// set the metrics extractors for testing
	originalMetricsExtractors := metricsExtractors
	metricsExtractors = []extractors.MetricExtractor{}
	metricsExtractors = append(metricsExtractors, extractors.NewCPUMetricExtractor(zap.NewNop()))
	metricsExtractors = append(metricsExtractors, extractors.NewMemMetricExtractor(zap.NewNop()))
	metricsExtractors = append(metricsExtractors, extractors.NewDiskIOMetricExtractor(zap.NewNop()))
	metricsExtractors = append(metricsExtractors, extractors.NewNetMetricExtractor(zap.NewNop()))
	metricsExtractors = append(metricsExtractors, extractors.NewFileSystemMetricExtractor(zap.NewNop()))

	containerInfos := testutils.LoadContainerInfo(t, "./extractors/testdata/CurInfoContainer.json")
	podInfos := testutils.LoadContainerInfo(t, "./extractors/testdata/InfoPod.json")
	containerInfos = append(containerInfos, podInfos...)
	containerInContainerInfos := testutils.LoadContainerInfo(t, "./extractors/testdata/ContainerInContainer.json")
	containerInfos = append(containerInfos, containerInContainerInfos...)
	mInfo := testutils.MockCPUMemInfo{}
	metrics := processContainers(containerInfos, mInfo, "eks", zap.NewNop())
	assert.Equal(t, 3, len(metrics))

	// restore the original value of metrics extractors
	metricsExtractors = originalMetricsExtractors
}
