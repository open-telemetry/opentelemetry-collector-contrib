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
	"time"

	"github.com/aws/aws-sdk-go/aws"
	awsec2metadata "github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"

	"go.uber.org/zap"
)

type EC2Metadata struct {
	logger          *zap.Logger
	refreshInterval time.Duration
	instanceID      string
	instanceType    string
	shutdownC       chan bool
}

func NewEC2Metadata(refreshInterval time.Duration, logger *zap.Logger) *EC2Metadata {
	emd := &EC2Metadata{
		refreshInterval: refreshInterval,
		shutdownC:       make(chan bool),
		logger:          logger,
	}

	emd.refresh()

	shouldRefresh := func() bool {
		//stop the refresh once we get instance ID and type successfully
		return emd.instanceID == "" || emd.instanceType == ""
	}

	go refreshUntil(emd.refresh, emd.refreshInterval, shouldRefresh, emd.shutdownC)

	return emd
}

func (emd *EC2Metadata) getAwsMetadata() awsec2metadata.EC2InstanceIdentityDocument {
	emd.logger.Info("Fetch instance id and type from ec2 metadata")
	sess, err := session.NewSession(&aws.Config{})
	if err != nil {
		emd.logger.Error("Failed to set up new session to call ec2 api")
		return awsec2metadata.EC2InstanceIdentityDocument{}
	}
	client := awsec2metadata.New(sess)
	doc, err := client.GetInstanceIdentityDocument()
	if err != nil {
		emd.logger.Error("Failed to get ec2 metadata", zap.Error(err))
		return awsec2metadata.EC2InstanceIdentityDocument{}
	}

	return doc
}

func (emd *EC2Metadata) refresh() {
	metadata := emd.getAwsMetadata()
	emd.instanceID = metadata.InstanceID
	emd.instanceType = metadata.InstanceType
}

func (emd *EC2Metadata) Shutdown() {
	close(emd.shutdownC)
}

func (emd *EC2Metadata) GetInstanceID() string {
	return emd.instanceID
}

func (emd *EC2Metadata) GetInstanceType() string {
	return emd.instanceType
}
