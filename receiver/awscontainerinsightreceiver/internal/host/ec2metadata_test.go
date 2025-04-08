// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package host

import (
	"context"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

type mockMetadataClient func(ctx context.Context, params *imds.GetInstanceIdentityDocumentInput, optFns ...func(*imds.Options)) (*imds.GetInstanceIdentityDocumentOutput, error)

func (m mockMetadataClient) GetInstanceIdentityDocument(ctx context.Context, params *imds.GetInstanceIdentityDocumentInput, optFns ...func(*imds.Options)) (*imds.GetInstanceIdentityDocumentOutput, error) {
	return m(ctx, params, optFns...)
}

func TestEC2Metadata(t *testing.T) {
	tests := []struct {
		name                 string
		client               func(t *testing.T) metadataClient
		expectedInstanceID   string
		expectedInstanceType string
		expectedPrivateIP    string
		expectedRegion       string
	}{
		{
			name: "Able to retrieve instance metadata",
			client: func(t *testing.T) metadataClient {
				return mockMetadataClient(func(_ context.Context, _ *imds.GetInstanceIdentityDocumentInput, _ ...func(*imds.Options)) (*imds.GetInstanceIdentityDocumentOutput, error) {
					t.Helper()
					return &imds.GetInstanceIdentityDocumentOutput{
						InstanceIdentityDocument: imds.InstanceIdentityDocument{
							Region:       "us-west-2",
							InstanceID:   "i-abcd1234",
							InstanceType: "c4.xlarge",
							PrivateIP:    "79.168.255.0",
						},
					}, nil
				})
			},
			expectedInstanceID:   "i-abcd1234",
			expectedInstanceType: "c4.xlarge",
			expectedRegion:       "us-west-2",
			expectedPrivateIP:    "79.168.255.0",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			e := &ec2Metadata{
				logger:           zap.NewNop(),
				client:           test.client(t),
				refreshInterval:  3 * time.Millisecond,
				instanceIDReadyC: make(chan bool),
				instanceIPReadyC: make(chan bool),
			}
			e.refresh(context.Background())

			assert.Equal(t, test.expectedInstanceID, e.getInstanceID())
			assert.Equal(t, test.expectedInstanceType, e.getInstanceType())
			assert.Equal(t, test.expectedRegion, e.getRegion())
			assert.Equal(t, test.expectedPrivateIP, e.getInstanceIP())
		})
	}
}
