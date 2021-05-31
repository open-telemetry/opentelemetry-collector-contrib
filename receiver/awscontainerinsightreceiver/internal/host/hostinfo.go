// Copyright  OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package host

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/awsutil"
)

// Info contains information about a host
type Info struct {
	cancel           context.CancelFunc
	logger           *zap.Logger
	awsSession       *session.Session
	refreshInterval  time.Duration
	instanceIDReadyC chan bool // close of this channel indicates instance ID is ready

	ebsVolumeReadyC chan bool // close of this channel indicates ebsVolume is initialized. It is used only in test
	ec2TagsReadyC   chan bool // close of this channel indicates ec2Tags is initialized. It is used only in test

	nodeCapacity NodeCapacityProvider
	ec2Metadata  EC2MetadataProvider
	ebsVolume    EBSVolumeProvider
	ec2Tags      EC2TagsProvider

	awsSessionCreator   func(*zap.Logger, awsutil.ConnAttr, *awsutil.AWSSessionSettings) (*aws.Config, *session.Session, error)
	nodeCapacityCreator func(*zap.Logger, ...nodeCapacityOption) (NodeCapacityProvider, error)
	ec2MetadataCreator  func(context.Context, *session.Session, time.Duration, chan bool, *zap.Logger, ...ec2MetadataOption) EC2MetadataProvider
	ebsVolumeCreator    func(context.Context, *session.Session, string, string, time.Duration, *zap.Logger, ...ebsVolumeOption) EBSVolumeProvider
	ec2TagsCreator      func(context.Context, *session.Session, string, time.Duration, *zap.Logger, ...ec2TagsOption) EC2TagsProvider
}

type machineInfoOption func(*Info)

// NewInfo creates a new Info struct
func NewInfo(refreshInterval time.Duration, logger *zap.Logger, options ...machineInfoOption) (*Info, error) {
	ctx, cancel := context.WithCancel(context.Background())
	mInfo := &Info{
		cancel:           cancel,
		refreshInterval:  refreshInterval,
		instanceIDReadyC: make(chan bool),
		logger:           logger,

		awsSessionCreator:   awsutil.GetAWSConfigSession,
		nodeCapacityCreator: NewNodeCapacity,
		ec2MetadataCreator:  NewEC2Metadata,
		ebsVolumeCreator:    NewEBSVolume,
		ec2TagsCreator:      NewEC2Tags,

		// used in test only
		ebsVolumeReadyC: make(chan bool),
		ec2TagsReadyC:   make(chan bool),
	}

	for _, opt := range options {
		opt(mInfo)
	}

	nodeCapacity, err := mInfo.nodeCapacityCreator(logger)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize NodeCapacity: %v", err)
	}
	mInfo.nodeCapacity = nodeCapacity

	defaultSessionConfig := awsutil.CreateDefaultSessionConfig()
	_, session, err := mInfo.awsSessionCreator(logger, &awsutil.Conn{}, &defaultSessionConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create aws session: %v", err)
	}
	mInfo.awsSession = session

	mInfo.ec2Metadata = mInfo.ec2MetadataCreator(ctx, session, refreshInterval, mInfo.instanceIDReadyC, logger)

	go mInfo.lazyInitEBSVolume(ctx)
	go mInfo.lazyInitEC2Tags(ctx)
	return mInfo, nil
}

func (m *Info) lazyInitEBSVolume(ctx context.Context) {
	//wait until the instance id is ready
	<-m.instanceIDReadyC
	//Because ebs volumes only change occasionally, we refresh every 5 collection intervals to reduce ec2 api calls
	m.ebsVolume = m.ebsVolumeCreator(ctx, m.awsSession, m.GetInstanceID(), m.GetRegion(),
		5*m.refreshInterval, m.logger)
	close(m.ebsVolumeReadyC)
}

func (m *Info) lazyInitEC2Tags(ctx context.Context) {
	//wait until the instance id is ready
	<-m.instanceIDReadyC
	m.ec2Tags = m.ec2TagsCreator(ctx, m.awsSession, m.GetInstanceID(), m.refreshInterval, m.logger)
	close(m.ec2TagsReadyC)
}

// GetInstanceID returns the ec2 instance id for the host
func (m *Info) GetInstanceID() string {
	return m.ec2Metadata.GetInstanceID()
}

// GetInstanceType returns the ec2 instance type for the host
func (m *Info) GetInstanceType() string {
	return m.ec2Metadata.GetInstanceType()
}

// GetRegion returns the region for the host
func (m *Info) GetRegion() string {
	return m.ec2Metadata.GetRegion()
}

// GetNumCores returns the number of cpu cores on the host
func (m *Info) GetNumCores() int64 {
	return m.nodeCapacity.GetNumCores()
}

// GetMemoryCapacity returns the total memory (in bytes) on the host
func (m *Info) GetMemoryCapacity() int64 {
	return m.nodeCapacity.GetMemoryCapacity()
}

// GetEBSVolumeID returns the ebs volume id corresponding to the given device name
func (m *Info) GetEBSVolumeID(devName string) string {
	if m.ebsVolume != nil {
		return m.ebsVolume.GetEBSVolumeID(devName)
	}

	return ""
}

// GetClusterName returns the cluster name associated with the host
func (m *Info) GetClusterName() string {
	if m.ec2Tags != nil {
		return m.ec2Tags.GetClusterName()
	}

	return ""
}

// GetAutoScalingGroupName returns the auto scaling group associated with the host
func (m *Info) GetAutoScalingGroupName() string {
	if m.ec2Tags != nil {
		return m.ec2Tags.GetAutoScalingGroupName()
	}

	return ""
}

// Shutdown stops the host Info
func (m *Info) Shutdown() {
	m.cancel()
}
