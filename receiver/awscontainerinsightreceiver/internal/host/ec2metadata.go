// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package host // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/host"

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
	"go.uber.org/zap"
)

type metadataClient interface {
	GetInstanceIdentityDocument(ctx context.Context, params *imds.GetInstanceIdentityDocumentInput, optFns ...func(*imds.Options)) (*imds.GetInstanceIdentityDocumentOutput, error)
}

type ec2MetadataProvider interface {
	getInstanceID() string
	getInstanceType() string
	getRegion() string
	getInstanceIP() string
}

type ec2Metadata struct {
	logger           *zap.Logger
	client           metadataClient
	refreshInterval  time.Duration
	instanceID       string
	instanceType     string
	instanceIP       string
	region           string
	instanceIDReadyC chan bool
	instanceIPReadyC chan bool
}

type ec2MetadataOption func(*ec2Metadata)

func newEC2Metadata(ctx context.Context, cfg aws.Config, refreshInterval time.Duration,
	instanceIDReadyC chan bool, instanceIPReadyC chan bool, logger *zap.Logger, options ...ec2MetadataOption,
) ec2MetadataProvider {
	emd := &ec2Metadata{
		client:           imds.NewFromConfig(cfg),
		refreshInterval:  refreshInterval,
		instanceIDReadyC: instanceIDReadyC,
		instanceIPReadyC: instanceIPReadyC,
		logger:           logger,
	}

	for _, opt := range options {
		opt(emd)
	}

	shouldRefresh := func() bool {
		// stop the refresh once we get instance ID and type successfully
		return emd.instanceID == "" || emd.instanceType == "" || emd.instanceIP == ""
	}

	go RefreshUntil(ctx, emd.refresh, emd.refreshInterval, shouldRefresh, 0)

	return emd
}

func (emd *ec2Metadata) refresh(ctx context.Context) {
	emd.logger.Info("Fetch instance id and type from ec2 metadata")

	resp, err := emd.client.GetInstanceIdentityDocument(ctx, &imds.GetInstanceIdentityDocumentInput{})
	if err != nil {
		emd.logger.Error("Failed to get ec2 metadata", zap.Error(err))
		return
	}

	emd.instanceID = resp.InstanceIdentityDocument.InstanceID
	emd.instanceType = resp.InstanceIdentityDocument.InstanceType
	emd.region = resp.InstanceIdentityDocument.Region
	emd.instanceIP = resp.InstanceIdentityDocument.PrivateIP

	// notify ec2tags and ebsvolume that the instance id is ready
	if emd.instanceID != "" {
		close(emd.instanceIDReadyC)
	}
	// notify ecsinfo that the instance id is ready
	if emd.instanceIP != "" {
		close(emd.instanceIPReadyC)
	}
}

func (emd *ec2Metadata) getInstanceID() string {
	return emd.instanceID
}

func (emd *ec2Metadata) getInstanceType() string {
	return emd.instanceType
}

func (emd *ec2Metadata) getRegion() string {
	return emd.region
}

func (emd *ec2Metadata) getInstanceIP() string {
	return emd.instanceIP
}
