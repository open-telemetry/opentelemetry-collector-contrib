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

	awsec2metadata "github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
	"go.uber.org/zap"
)

type metadataClient interface {
	GetInstanceIdentityDocumentWithContext(ctx context.Context) (awsec2metadata.EC2InstanceIdentityDocument, error)
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

func newEC2Metadata(ctx context.Context, session *session.Session, refreshInterval time.Duration,
	instanceIDReadyC chan bool, instanceIPReadyC chan bool, logger *zap.Logger, options ...ec2MetadataOption) ec2MetadataProvider {
	emd := &ec2Metadata{
		client:           awsec2metadata.New(session),
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

	doc, err := emd.client.GetInstanceIdentityDocumentWithContext(ctx)
	if err != nil {
		emd.logger.Error("Failed to get ec2 metadata", zap.Error(err))
		return
	}

	emd.instanceID = doc.InstanceID
	emd.instanceType = doc.InstanceType
	emd.region = doc.Region
	emd.instanceIP = doc.PrivateIP

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
