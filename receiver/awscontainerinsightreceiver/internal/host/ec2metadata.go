// Copyright The OpenTelemetry Authors
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

package host // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/host"

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/awsutil"
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
	logger               *zap.Logger
	clientIMDSV2Only     metadataClient
	clientIMDSV1Fallback metadataClient
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

func newEC2Metadata(ctx context.Context, cfg *aws.Config, refreshInterval time.Duration,
	instanceIDReadyC chan bool, instanceIPReadyC chan bool, localMode bool, logger *zap.Logger, options ...ec2MetadataOption) ec2MetadataProvider {
	clientIMDSV2Only, clientIMDSV1Fallback := awsutil.CreateIMDSV2AndFallbackClient(*cfg)
	emd := &ec2Metadata{
		clientIMDSV2Only:     clientIMDSV2Only,
		clientIMDSV1Fallback: clientIMDSV1Fallback,
		refreshInterval:      refreshInterval,
		instanceIDReadyC:     instanceIDReadyC,
		instanceIPReadyC:     instanceIPReadyC,
		localMode:            localMode,
		logger:               logger,
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

	err := emd.callIMDSClient(emd.clientIMDSV2Only)
	if err != nil {
		emd.logger.Error("Failed to get ec2 metadata via imdsv2", zap.Error(err))
		err = emd.callIMDSClient(emd.clientIMDSV1Fallback)
		if err != nil {
			emd.logger.Error("Failed to get ec2 metadata via imdsv1", zap.Error(err))
		}
		return
	}
}

func (emd *ec2Metadata) callIMDSClient(client metadataClient) error {
	getInstanceDocumentInput := imds.GetInstanceIdentityDocumentInput{}
	instanceDocument, err := client.GetInstanceIdentityDocument(context.Background(), &getInstanceDocumentInput)
	if err != nil {
		emd.logger.Warn("Fetch identity document from EC2 metadata fail: %v", zap.Error(err))
		return err
	}
	emd.instanceID = instanceDocument.InstanceID
	emd.instanceType = instanceDocument.InstanceType
	emd.region = instanceDocument.Region
	emd.instanceIP = instanceDocument.PrivateIP

	// notify ec2tags and ebsvolume that the instance id is ready
	if emd.instanceID != "" {
		close(emd.instanceIDReadyC)
	}
	// notify ecsinfo that the instance id is ready
	if emd.instanceIP != "" {
		close(emd.instanceIPReadyC)
	}
	return nil
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
