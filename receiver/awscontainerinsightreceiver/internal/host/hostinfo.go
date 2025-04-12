// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package host // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/host"

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/awsutil"
)

// Info contains information about a host
type Info struct {
	cancel                context.CancelFunc
	logger                *zap.Logger
	awsConfig             aws.Config
	refreshInterval       time.Duration
	containerOrchestrator string
	instanceIDReadyC      chan bool // close of this channel indicates instance ID is ready
	instanceIPReadyC      chan bool // close of this channel indicates instance Ip is ready

	ebsVolumeReadyC chan bool // close of this channel indicates ebsVolume is initialized. It is used only in test
	ec2TagsReadyC   chan bool // close of this channel indicates ec2Tags is initialized. It is used only in test

	nodeCapacity nodeCapacityProvider
	ec2Metadata  ec2MetadataProvider
	ebsVolume    ebsVolumeProvider
	ec2Tags      ec2TagsProvider

	awsConfigCreator    func(*zap.Logger, *awsutil.AWSSessionSettings) (aws.Config, error)
	nodeCapacityCreator func(*zap.Logger, ...nodeCapacityOption) (nodeCapacityProvider, error)
	ec2MetadataCreator  func(context.Context, aws.Config, time.Duration, chan bool, chan bool, *zap.Logger, ...ec2MetadataOption) ec2MetadataProvider
	ebsVolumeCreator    func(context.Context, aws.Config, string, string, time.Duration, *zap.Logger, ...ebsVolumeOption) ebsVolumeProvider
	ec2TagsCreator      func(context.Context, aws.Config, string, string, string, time.Duration, *zap.Logger, ...ec2TagsOption) ec2TagsProvider
}

type machineInfoOption func(*Info)

// NewInfo creates a new Info struct
func NewInfo(containerOrchestrator string, refreshInterval time.Duration, logger *zap.Logger, options ...machineInfoOption) (*Info, error) {
	ctx, cancel := context.WithCancel(context.Background())
	mInfo := &Info{
		cancel:           cancel,
		refreshInterval:  refreshInterval,
		instanceIDReadyC: make(chan bool),
		instanceIPReadyC: make(chan bool),
		logger:           logger,

		containerOrchestrator: containerOrchestrator,
		awsConfigCreator:      awsutil.GetAWSConfig,
		nodeCapacityCreator:   newNodeCapacity,
		ec2MetadataCreator:    newEC2Metadata,
		ebsVolumeCreator:      newEBSVolume,
		ec2TagsCreator:        newEC2Tags,

		// used in test only
		ebsVolumeReadyC: make(chan bool),
		ec2TagsReadyC:   make(chan bool),
	}

	for _, opt := range options {
		opt(mInfo)
	}

	nodeCapacity, err := mInfo.nodeCapacityCreator(logger)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize NodeCapacity: %w", err)
	}
	mInfo.nodeCapacity = nodeCapacity

	defaultSessionConfig := awsutil.CreateDefaultSessionConfig()
	cfg, err := mInfo.awsConfigCreator(logger, &defaultSessionConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create aws session: %w", err)
	}

	mInfo.ec2Metadata = mInfo.ec2MetadataCreator(ctx, cfg, refreshInterval, mInfo.instanceIDReadyC, mInfo.instanceIPReadyC, logger)

	go mInfo.lazyInitEBSVolume(ctx)
	go mInfo.lazyInitEC2Tags(ctx)
	return mInfo, nil
}

func (m *Info) lazyInitEBSVolume(ctx context.Context) {
	// wait until the instance id is ready
	<-m.instanceIDReadyC
	// Because ebs volumes only change occasionally, we refresh every 5 collection intervals to reduce ec2 api calls
	m.ebsVolume = m.ebsVolumeCreator(ctx, m.awsConfig, m.GetInstanceID(), m.GetRegion(),
		5*m.refreshInterval, m.logger)
	close(m.ebsVolumeReadyC)
}

func (m *Info) lazyInitEC2Tags(ctx context.Context) {
	// wait until the instance id is ready
	<-m.instanceIDReadyC
	m.ec2Tags = m.ec2TagsCreator(ctx, m.awsConfig, m.GetInstanceID(), m.GetRegion(), m.containerOrchestrator, m.refreshInterval, m.logger)
	close(m.ec2TagsReadyC)
}

// GetInstanceID returns the ec2 instance id for the host
func (m *Info) GetInstanceID() string {
	return m.ec2Metadata.getInstanceID()
}

// GetInstanceType returns the ec2 instance type for the host
func (m *Info) GetInstanceType() string {
	return m.ec2Metadata.getInstanceType()
}

// GetRegion returns the region for the host
func (m *Info) GetRegion() string {
	return m.ec2Metadata.getRegion()
}

// GetInstanceIP returns the IP address of the host
func (m *Info) GetInstanceIP() string {
	return m.ec2Metadata.getInstanceIP()
}

// GetNumCores returns the number of cpu cores on the host
func (m *Info) GetNumCores() int64 {
	return m.nodeCapacity.getNumCores()
}

// GetMemoryCapacity returns the total memory (in bytes) on the host
func (m *Info) GetMemoryCapacity() int64 {
	return m.nodeCapacity.getMemoryCapacity()
}

// GetEBSVolumeID returns the ebs volume id corresponding to the given device name
func (m *Info) GetEBSVolumeID(devName string) string {
	if m.ebsVolume != nil {
		return m.ebsVolume.getEBSVolumeID(devName)
	}

	return ""
}

// GetClusterName returns the cluster name associated with the host
func (m *Info) GetClusterName() string {
	if m.ec2Tags != nil {
		return m.ec2Tags.getClusterName()
	}

	return ""
}

// GetInstanceIPReadyC returns the channel to show the status of host IP
func (m *Info) GetInstanceIPReadyC() chan bool {
	return m.instanceIPReadyC
}

// GetAutoScalingGroupName returns the auto scaling group associated with the host
func (m *Info) GetAutoScalingGroupName() string {
	if m.ec2Tags != nil {
		return m.ec2Tags.getAutoScalingGroupName()
	}

	return ""
}

// ExtractEbsIDsUsedByKubernetes extracts the ebs volume id used by kubernetes cluster from host mount file
func (m *Info) ExtractEbsIDsUsedByKubernetes() map[string]string {
	if m.ebsVolume != nil {
		return m.ebsVolume.extractEbsIDsUsedByKubernetes()
	}
	return map[string]string{}
}

// Shutdown stops the host Info
func (m *Info) Shutdown() {
	m.cancel()
}
