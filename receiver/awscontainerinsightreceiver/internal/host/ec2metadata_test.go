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
	"errors"
	"testing"
	"time"

	awsec2metadata "github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/awstesting/mock"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

type mockMetadataClient struct {
	count int
}

func (m *mockMetadataClient) GetInstanceIdentityDocumentWithContext(ctx context.Context) (awsec2metadata.EC2InstanceIdentityDocument, error) {
	m.count++
	if m.count == 1 {
		return awsec2metadata.EC2InstanceIdentityDocument{}, errors.New("error")
	}

	return awsec2metadata.EC2InstanceIdentityDocument{
		Region:       "us-west-2",
		InstanceID:   "i-abcd1234",
		InstanceType: "c4.xlarge",
	}, nil
}

func TestEC2Metadata(t *testing.T) {
	ctx := context.Background()
	sess := mock.Session
	instanceIDReadyC := make(chan bool)
	clientOption := func(e *EC2Metadata) {
		e.client = &mockMetadataClient{}
	}
	ec2metadata := NewEC2Metadata(ctx, sess, 3*time.Millisecond, instanceIDReadyC, zap.NewNop(), clientOption)
	assert.NotNil(t, ec2metadata)

	<-instanceIDReadyC
	assert.Equal(t, "i-abcd1234", ec2metadata.GetInstanceID())
	assert.Equal(t, "c4.xlarge", ec2metadata.GetInstanceType())
	assert.Equal(t, "us-west-2", ec2metadata.GetRegion())
}
