// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package host // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/host"

import (
	"context"
	"log"
	"time"

	"github.com/amazon-contributing/opentelemetry-collector-contrib/extension/awsmiddleware"
	override "github.com/amazon-contributing/opentelemetry-collector-contrib/override/aws"
	"github.com/aws/aws-sdk-go/aws"
	awsec2metadata "github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
	"go.uber.org/zap"
)

type metadataClient interface {
	GetInstanceIdentityDocument() (awsec2metadata.EC2InstanceIdentityDocument, error)
}

type ec2MetadataProvider interface {
	getInstanceID() string
	getInstanceType() string
	getRegion() string
	getInstanceIP() string
}

type ec2Metadata struct {
	logger               *zap.Logger
	client               metadataClient
	clientFallbackEnable metadataClient
	refreshInterval      time.Duration
	instanceID           string
	instanceType         string
	instanceIP           string
	region               string
	instanceIDReadyC     chan bool
	instanceIPReadyC     chan bool
	localMode            bool
}

type ec2MetadataOption func(*ec2Metadata)

func newEC2Metadata(ctx context.Context, session *session.Session, refreshInterval time.Duration,
	instanceIDReadyC chan bool, instanceIPReadyC chan bool, localMode bool, imdsRetries int, logger *zap.Logger, configurer *awsmiddleware.Configurer, options ...ec2MetadataOption,
) ec2MetadataProvider {
	emd := &ec2Metadata{
		client: awsec2metadata.New(session, &aws.Config{
			Retryer:                   override.NewIMDSRetryer(imdsRetries),
			EC2MetadataEnableFallback: aws.Bool(false),
		}),
		clientFallbackEnable: awsec2metadata.New(session, &aws.Config{}),
		refreshInterval:      refreshInterval,
		instanceIDReadyC:     instanceIDReadyC,
		instanceIPReadyC:     instanceIPReadyC,
		localMode:            localMode,
		logger:               logger,
	}
	if configurer != nil {
		err := configurer.Configure(awsmiddleware.SDKv1(&emd.client.(*awsec2metadata.EC2Metadata).Handlers))
		if err != nil {
			log.Println("There was a problem configuring middleware on ec2 client")
		}
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

func (emd *ec2Metadata) refresh(_ context.Context) {
	if emd.localMode {
		emd.logger.Debug("Running EC2MetadataProvider in local mode.  Skipping EC2 metadata fetch")
		return
	}
	emd.logger.Info("Fetch instance id and type from ec2 metadata")

	doc, err := emd.client.GetInstanceIdentityDocument()
	if err != nil {
		docInner, errInner := emd.clientFallbackEnable.GetInstanceIdentityDocument()
		if errInner != nil {
			emd.logger.Error("Failed to get ec2 metadata", zap.Error(err))
			return
		}
		emd.instanceID = docInner.InstanceID
		emd.instanceType = docInner.InstanceType
		emd.region = docInner.Region
		emd.instanceIP = docInner.PrivateIP
	} else {
		emd.instanceID = doc.InstanceID
		emd.instanceType = doc.InstanceType
		emd.region = doc.Region
		emd.instanceIP = doc.PrivateIP
	}

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
