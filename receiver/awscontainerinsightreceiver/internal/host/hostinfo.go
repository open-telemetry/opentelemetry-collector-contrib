// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package host // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/host"

import (
	"context"
	"fmt"
	"time"

	"github.com/amazon-contributing/opentelemetry-collector-contrib/extension/awsmiddleware"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/awsutil"
)

// Info contains information about a host
type Info struct {
	cancel                context.CancelFunc
	logger                *zap.Logger
	awsSession            *session.Session
	refreshInterval       time.Duration
	containerOrchestrator string
	clusterName           string
	isSystemdEnabled      bool      // flag to indicate if agent is running on systemd in EC2 environment
	instanceIDReadyC      chan bool // close of this channel indicates instance ID is ready
	instanceIPReadyC      chan bool // close of this channel indicates instance Ip is ready

	ebsVolumeReadyC chan bool // close of this channel indicates ebsVolume is initialized. It is used only in test
	ec2TagsReadyC   chan bool // close of this channel indicates ec2Tags is initialized. It is used only in test

	nodeCapacity nodeCapacityProvider
	ec2Metadata  ec2MetadataProvider
	ebsVolume    ebsVolumeProvider
	ec2Tags      ec2TagsProvider

	awsSessionCreator   func(*zap.Logger, awsutil.ConnAttr, *awsutil.AWSSessionSettings) (*aws.Config, *session.Session, error)
	nodeCapacityCreator func(*zap.Logger, ...Option) (nodeCapacityProvider, error)
	ec2MetadataCreator  func(context.Context, *session.Session, time.Duration, chan bool, chan bool, bool, int, *zap.Logger, ...ec2MetadataOption) ec2MetadataProvider
	ebsVolumeCreator    func(context.Context, *session.Session, string, string, time.Duration, *zap.Logger, ...ebsVolumeOption) ebsVolumeProvider
	ec2TagsCreator      func(context.Context, *session.Session, string, string, string, time.Duration, *zap.Logger, ...ec2TagsOption) ec2TagsProvider
}

type Option func(any)

func WithClusterName(name string) Option {
	return func(info any) {
		if i, ok := info.(*Info); ok {
			i.clusterName = name
		}
	}
}

func WithSystemdEnabled(enabled bool) Option {
	return func(info any) {
		switch i := info.(type) {
		case *Info:
			i.isSystemdEnabled = enabled
		case *nodeCapacity:
			i.isSystemdEnabled = enabled
		}
	}
}

func withConfigurer[T any](configurer *awsmiddleware.Configurer, creator func(configurer *awsmiddleware.Configurer) T) T {
	return creator(configurer)
}

// NewInfo creates a new Info struct
func NewInfo(awsSessionSettings awsutil.AWSSessionSettings, containerOrchestrator string, refreshInterval time.Duration, logger *zap.Logger, configurer *awsmiddleware.Configurer, options ...Option) (*Info, error) {
	ctx, cancel := context.WithCancel(context.Background())
	mInfo := &Info{
		cancel:           cancel,
		refreshInterval:  refreshInterval,
		instanceIDReadyC: make(chan bool),
		instanceIPReadyC: make(chan bool),
		logger:           logger,

		containerOrchestrator: containerOrchestrator,
		awsSessionCreator:     awsutil.GetAWSConfigSession,
		nodeCapacityCreator:   newNodeCapacity,
		ec2MetadataCreator:    createEC2MetadataCreator(configurer),
		ebsVolumeCreator:      createEBSVolumeCreator(configurer),
		ec2TagsCreator:        createEC2TagsCreator(configurer),
		ebsVolumeReadyC:       make(chan bool),
		ec2TagsReadyC:         make(chan bool),
	}

	for _, opt := range options {
		opt(mInfo)
	}

	nodeCapacity, err := mInfo.nodeCapacityCreator(logger, WithSystemdEnabled(mInfo.isSystemdEnabled))
	if err != nil {
		return nil, fmt.Errorf("failed to initialize NodeCapacity: %w", err)
	}
	mInfo.nodeCapacity = nodeCapacity

	_, session, err := mInfo.awsSessionCreator(logger, &awsutil.Conn{}, &awsSessionSettings)
	if err != nil {
		return nil, fmt.Errorf("failed to create aws session: %w", err)
	}
	mInfo.awsSession = session

	mInfo.ec2Metadata = mInfo.ec2MetadataCreator(ctx, session, refreshInterval, mInfo.instanceIDReadyC, mInfo.instanceIPReadyC, awsSessionSettings.LocalMode, awsSessionSettings.IMDSRetries, logger)

	go mInfo.lazyInitEBSVolume(ctx)
	go mInfo.lazyInitEC2Tags(ctx)
	return mInfo, nil
}

// createEC2MetadataCreator returns a function that creates an EC2 metadata provider.
// It uses the provided configurer to set up AWS middleware for the EC2 metadata client.
//
// The returned function is a factory that, when called, creates an EC2 metadata provider
// with the following capabilities:
// - Fetches EC2 instance metadata
// - Refreshes metadata at specified intervals
// - Notifies when instance ID and IP are ready via channels
// - Supports local mode and IMDS retries
// - Configures logging
// - Applies additional options for customization
//
// This layered approach allows for flexible configuration and dependency injection,
// making it easier to customize behavior and improve testability.
func createEC2MetadataCreator(configurer *awsmiddleware.Configurer) func(context.Context, *session.Session, time.Duration, chan bool, chan bool, bool, int, *zap.Logger, ...ec2MetadataOption) ec2MetadataProvider {
	return withConfigurer(configurer, func(c *awsmiddleware.Configurer) func(context.Context, *session.Session, time.Duration, chan bool, chan bool, bool, int, *zap.Logger, ...ec2MetadataOption) ec2MetadataProvider {
		return func(ctx context.Context, session *session.Session, refreshInterval time.Duration, instanceIDReadyC, instanceIPReadyC chan bool, localMode bool, imdsRetries int, logger *zap.Logger, options ...ec2MetadataOption) ec2MetadataProvider {
			return newEC2Metadata(ctx, session, refreshInterval, instanceIDReadyC, instanceIPReadyC, localMode, imdsRetries, logger, c, options...)
		}
	})
}

// createEBSVolumeCreator returns a function that creates an EBS volume provider.
// It uses the provided configurer to set up AWS middleware for the EBS volume client.
//
// The returned function is a factory that, when called, creates an EBS volume provider
// with the following capabilities:
// - Fetches EBS volume information for a specific EC2 instance
// - Refreshes volume information at specified intervals
// - Configures logging
// - Applies additional options for customization
//
// This approach allows for flexible configuration of the EBS volume provider,
// enabling easier testing and customization of behavior.
func createEBSVolumeCreator(configurer *awsmiddleware.Configurer) func(context.Context, *session.Session, string, string, time.Duration, *zap.Logger, ...ebsVolumeOption) ebsVolumeProvider {
	return withConfigurer(configurer, func(c *awsmiddleware.Configurer) func(context.Context, *session.Session, string, string, time.Duration, *zap.Logger, ...ebsVolumeOption) ebsVolumeProvider {
		return func(ctx context.Context, session *session.Session, instanceID, region string, refreshInterval time.Duration, logger *zap.Logger, options ...ebsVolumeOption) ebsVolumeProvider {
			return newEBSVolume(ctx, session, instanceID, region, refreshInterval, logger, c, options...)
		}
	})
}

// createEC2TagsCreator returns a function that creates an EC2 tags provider.
// It uses the provided configurer to set up AWS middleware for the EC2 tags client.
//
// The returned function is a factory that, when called, creates an EC2 tags provider
// with the following capabilities:
// - Fetches EC2 instance tags
// - Refreshes tag information at specified intervals
// - Supports container orchestrator-specific tag handling
// - Configures logging
// - Applies additional options for customization
//
// This design allows for flexible configuration of the EC2 tags provider,
// facilitating easier testing and customization of tag retrieval and processing.
func createEC2TagsCreator(configurer *awsmiddleware.Configurer) func(context.Context, *session.Session, string, string, string, time.Duration, *zap.Logger, ...ec2TagsOption) ec2TagsProvider {
	return withConfigurer(configurer, func(c *awsmiddleware.Configurer) func(context.Context, *session.Session, string, string, string, time.Duration, *zap.Logger, ...ec2TagsOption) ec2TagsProvider {
		return func(ctx context.Context, session *session.Session, instanceID, region, containerOrchestrator string, refreshInterval time.Duration, logger *zap.Logger, options ...ec2TagsOption) ec2TagsProvider {
			return newEC2Tags(ctx, session, instanceID, region, containerOrchestrator, refreshInterval, logger, c, options...)
		}
	})
}

func (m *Info) lazyInitEBSVolume(ctx context.Context) {
	// wait until the instance id is ready
	<-m.instanceIDReadyC
	// Because ebs volumes only change occasionally, we refresh every 5 collection intervals to reduce ec2 api calls
	m.ebsVolume = m.ebsVolumeCreator(ctx, m.awsSession, m.GetInstanceID(), m.GetRegion(),
		5*m.refreshInterval, m.logger)
	close(m.ebsVolumeReadyC)
}

func (m *Info) lazyInitEC2Tags(ctx context.Context) {
	// wait until the instance id is ready
	<-m.instanceIDReadyC
	m.ec2Tags = m.ec2TagsCreator(ctx, m.awsSession, m.GetInstanceID(), m.GetRegion(), m.containerOrchestrator, m.refreshInterval, m.logger)
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
	if m.clusterName != "" {
		return m.clusterName
	}
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
