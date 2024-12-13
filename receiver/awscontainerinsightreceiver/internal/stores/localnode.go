// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package stores // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/stores"

import (
	"fmt"

	"go.uber.org/zap"

	ci "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/containerinsight"
)

type LocalNodeDecorator struct {
	hostInfo              hostInfo
	version               string
	nodeName              string
	containerOrchestrator string
	ecsInfo               EcsInfo
	logger                *zap.Logger
	k8sDecorator          Decorator
}

type hostInfo interface {
	GetClusterName() string
	GetEBSVolumeID(string) string
	ExtractEbsIDsUsedByKubernetes() map[string]string
	GetInstanceID() string
	GetInstanceType() string
	GetAutoScalingGroupName() string
}

type EcsInfo interface {
	GetContainerInstanceID() string
	GetClusterName() string
}

type Decorator interface {
	Decorate(CIMetric) CIMetric
	Shutdown() error
}

func NewLocalNodeDecorator(logger *zap.Logger, containerOrchestrator string, hostInfo hostInfo, hostName string, options ...Option) (*LocalNodeDecorator, error) {
	if hostName == "" && containerOrchestrator == ci.EKS {
		return nil, fmt.Errorf("missing environment variable %s. Please check your deployment YAML config or agent config", ci.HostName)
	}

	d := &LocalNodeDecorator{
		hostInfo:              hostInfo,
		version:               "0",
		nodeName:              hostName,
		containerOrchestrator: containerOrchestrator,
		logger:                logger,
	}

	for _, option := range options {
		option(d)
	}

	return d, nil
}

type Option func(decorator *LocalNodeDecorator)

func WithK8sDecorator(d Decorator) Option {
	return func(ld *LocalNodeDecorator) {
		ld.k8sDecorator = d
	}
}

func WithECSInfo(f EcsInfo) Option {
	return func(c *LocalNodeDecorator) {
		c.ecsInfo = f
	}
}

func (d *LocalNodeDecorator) Decorate(m CIMetric) CIMetric {
	result := m

	ebsVolumeIDsUsedAsPV := d.hostInfo.ExtractEbsIDsUsedByKubernetes()
	tags := m.GetTags()
	d.addEbsVolumeInfo(tags, ebsVolumeIDsUsedAsPV)

	// add version
	tags[ci.Version] = d.version

	// add NodeName for node, pod and container
	metricType := tags[ci.MetricType]
	if d.nodeName != "" && (ci.IsNode(metricType) || ci.IsInstance(metricType) ||
		ci.IsPod(metricType) || ci.IsContainer(metricType)) {
		tags[ci.NodeNameKey] = d.nodeName
	}

	// add instance id and type
	if instanceID := d.hostInfo.GetInstanceID(); instanceID != "" {
		tags[ci.InstanceID] = instanceID
	}
	if instanceType := d.hostInfo.GetInstanceType(); instanceType != "" {
		tags[ci.InstanceType] = instanceType
	}

	// add scaling group name
	tags[ci.AutoScalingGroupNameKey] = d.hostInfo.GetAutoScalingGroupName()

	// add ECS cluster name and container instance id
	if d.containerOrchestrator == ci.ECS {
		d.addECSResources(m)
	}

	// add tags for EKS
	if d.containerOrchestrator == ci.EKS {
		tags[ci.ClusterNameKey] = d.hostInfo.GetClusterName()

		if d.k8sDecorator != nil {
			result = d.k8sDecorator.Decorate(m)
		}
	}

	return result
}

func (d *LocalNodeDecorator) addEbsVolumeInfo(tags map[string]string, ebsVolumeIDsUsedAsPV map[string]string) {
	deviceName, ok := tags[ci.DiskDev]
	if !ok {
		return
	}

	if d.hostInfo != nil {
		if volID := d.hostInfo.GetEBSVolumeID(deviceName); volID != "" {
			tags[ci.HostEbsVolumeID] = volID
		}
	}

	if tags[ci.MetricType] == ci.TypeContainerFS || tags[ci.MetricType] == ci.TypeNodeFS ||
		tags[ci.MetricType] == ci.TypeNodeDiskIO || tags[ci.MetricType] == ci.TypeContainerDiskIO {
		if volID := ebsVolumeIDsUsedAsPV[deviceName]; volID != "" {
			tags[ci.EbsVolumeID] = volID
		}
	}
}

func (d *LocalNodeDecorator) addECSResources(m CIMetric) {
	tags := m.GetTags()

	if d.ecsInfo.GetClusterName() == "" {
		d.logger.Warn("Can't get cluster name")
	} else {
		tags[ci.ClusterNameKey] = d.ecsInfo.GetClusterName()
	}

	if d.ecsInfo.GetContainerInstanceID() == "" {
		d.logger.Warn("Can't get containerInstanceId")
	} else {
		tags[ci.ContainerInstanceIDKey] = d.ecsInfo.GetContainerInstanceID()
	}

	TagMetricSource(m)
}

func (d *LocalNodeDecorator) Shutdown() error {
	return nil
}
