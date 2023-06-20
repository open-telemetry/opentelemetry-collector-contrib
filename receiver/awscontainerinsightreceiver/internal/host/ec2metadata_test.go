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

package host

import (
	"context"
	"errors"
	"testing"
	"time"

	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

type mockMetadataClient struct {
	count int
}

func (m *mockMetadataClient) GetInstanceIdentityDocument(ctx context.Context, params *imds.GetInstanceIdentityDocumentInput, optFns ...func(*imds.Options)) (*imds.GetInstanceIdentityDocumentOutput, error) {
	m.count++
	if m.count == 1 {
		return nil, errors.New("error")
	}
	return &imds.GetInstanceIdentityDocumentOutput{
		InstanceIdentityDocument: imds.InstanceIdentityDocument{
			Region:       "us-west-2",
			InstanceID:   "i-abcd1234",
			InstanceType: "c4.xlarge",
			PrivateIP:    "79.168.255.0",
		},
	}, nil
}

func TestEC2Metadata(t *testing.T) {
	ctx := context.Background()
	cfg, _ := awsConfig.LoadDefaultConfig(context.Background())
	instanceIDReadyC := make(chan bool)
	instanceIPReadyP := make(chan bool)
	clientOption := func(e *ec2Metadata) {
		e.clientIMDSV2Only = &mockMetadataClient{}
	}
	e := newEC2Metadata(ctx, &cfg, 3*time.Millisecond, instanceIDReadyC, instanceIPReadyP, false, zap.NewNop(), clientOption)
	assert.NotNil(t, e)

	<-instanceIDReadyC
	<-instanceIPReadyP
	assert.Equal(t, "i-abcd1234", e.getInstanceID())
	assert.Equal(t, "c4.xlarge", e.getInstanceType())
	assert.Equal(t, "us-west-2", e.getRegion())
	assert.Equal(t, "79.168.255.0", e.getInstanceIP())
}
