// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package host // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/host"

import (
	"context"
	"io"
	"strings"
	"time"

	override "github.com/amazon-contributing/opentelemetry-collector-contrib/override/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
	"go.uber.org/zap"
)

type metadataClient interface {
	GetInstanceIdentityDocument(ctx context.Context, params *imds.GetInstanceIdentityDocumentInput, optFns ...func(*imds.Options)) (*imds.GetInstanceIdentityDocumentOutput, error)
	GetMetadata(ctx context.Context, params *imds.GetMetadataInput, optFns ...func(*imds.Options)) (*imds.GetMetadataOutput, error)
}

type ec2MetadataProvider interface {
	getInstanceID() string
	getInstanceType() string
	getRegion() string
	getInstanceIP() string
	getNetworkInterfaceID(macAddress string) (string, error)
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
	networkInterfaceIDs  map[string]string
}

type ec2MetadataOption func(*ec2Metadata)

func newEC2Metadata(
	ctx context.Context,
	cfg aws.Config,
	refreshInterval time.Duration,
	instanceIDReadyC, instanceIPReadyC chan bool,
	localMode bool,
	imdsRetries int,
	logger *zap.Logger,
	options ...ec2MetadataOption,
) ec2MetadataProvider {
	emd := &ec2Metadata{
		client: imds.NewFromConfig(cfg, func(o *imds.Options) {
			o.Retryer = override.NewIMDSRetryer(imdsRetries)
			o.EnableFallback = aws.FalseTernary
		}),
		clientFallbackEnable: imds.NewFromConfig(cfg, func(o *imds.Options) {
			o.EnableFallback = aws.TrueTernary
		}),
		refreshInterval:     refreshInterval,
		instanceIDReadyC:    instanceIDReadyC,
		instanceIPReadyC:    instanceIPReadyC,
		localMode:           localMode,
		logger:              logger,
		networkInterfaceIDs: make(map[string]string),
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
	if emd.localMode {
		emd.logger.Debug("Running EC2MetadataProvider in local mode.  Skipping EC2 metadata fetch")
		return
	}
	emd.logger.Info("Fetch instance id and type from ec2 metadata")

	doc, err := emd.client.GetInstanceIdentityDocument(ctx, &imds.GetInstanceIdentityDocumentInput{})
	if err != nil {
		docInner, errInner := emd.clientFallbackEnable.GetInstanceIdentityDocument(ctx, &imds.GetInstanceIdentityDocumentInput{})
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

func (emd *ec2Metadata) getNetworkInterfaceID(macAddress string) (string, error) {
	// Check if we already have the ENI ID cached
	if eniID, exists := emd.networkInterfaceIDs[macAddress]; exists {
		return eniID, nil
	}

	// Load the ENI ID from metadata service
	eniID, err := emd.loadNetworkInterfaceID(macAddress)
	if err != nil {
		return "", err
	}

	// Cache the result
	emd.networkInterfaceIDs[macAddress] = eniID
	return eniID, nil
}

func (emd *ec2Metadata) loadNetworkInterfaceID(macAddress string) (string, error) {
	in := &imds.GetMetadataInput{Path: "network/interfaces/macs/" + macAddress + "/interface-id"}
	if out, err := emd.client.GetMetadata(context.Background(), in); err == nil {
		return readAndClose(out.Content)
	}
	out, err := emd.clientFallbackEnable.GetMetadata(context.Background(), in)
	if err != nil {
		return "", err
	}
	return readAndClose(out.Content)
}

func readAndClose(rc io.ReadCloser) (string, error) {
	defer func() { _ = rc.Close() }()
	body, err := io.ReadAll(rc)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(body)), nil
}
