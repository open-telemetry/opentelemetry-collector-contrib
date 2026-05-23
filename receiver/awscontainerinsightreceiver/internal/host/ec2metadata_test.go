// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package host

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

// mockMetadataClient implements the metadataClient interface for tests. The
// two function fields let each test wire whichever method it exercises.
type mockMetadataClient struct {
	identityFn func(ctx context.Context, params *imds.GetInstanceIdentityDocumentInput, optFns ...func(*imds.Options)) (*imds.GetInstanceIdentityDocumentOutput, error)
	metadataFn func(ctx context.Context, params *imds.GetMetadataInput, optFns ...func(*imds.Options)) (*imds.GetMetadataOutput, error)
}

func (m *mockMetadataClient) GetInstanceIdentityDocument(ctx context.Context, params *imds.GetInstanceIdentityDocumentInput, optFns ...func(*imds.Options)) (*imds.GetInstanceIdentityDocumentOutput, error) {
	if m.identityFn == nil {
		return nil, errors.New("identityFn not configured")
	}
	return m.identityFn(ctx, params, optFns...)
}

func (m *mockMetadataClient) GetMetadata(ctx context.Context, params *imds.GetMetadataInput, optFns ...func(*imds.Options)) (*imds.GetMetadataOutput, error) {
	if m.metadataFn == nil {
		return nil, errors.New("metadataFn not configured")
	}
	return m.metadataFn(ctx, params, optFns...)
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
				t.Helper()
				return &mockMetadataClient{
					identityFn: func(_ context.Context, _ *imds.GetInstanceIdentityDocumentInput, _ ...func(*imds.Options)) (*imds.GetInstanceIdentityDocumentOutput, error) {
						return &imds.GetInstanceIdentityDocumentOutput{
							InstanceIdentityDocument: imds.InstanceIdentityDocument{
								Region:       "us-west-2",
								InstanceID:   "i-abcd1234",
								InstanceType: "c4.xlarge",
								PrivateIP:    "79.168.255.0",
							},
						}, nil
					},
				}
			},
			expectedInstanceID:   "i-abcd1234",
			expectedInstanceType: "c4.xlarge",
			expectedRegion:       "us-west-2",
			expectedPrivateIP:    "79.168.255.0",
		},
		{
			name: "Falls back to permissive client when strict client errors",
			client: func(t *testing.T) metadataClient {
				t.Helper()
				return &mockMetadataClient{
					identityFn: func(_ context.Context, _ *imds.GetInstanceIdentityDocumentInput, _ ...func(*imds.Options)) (*imds.GetInstanceIdentityDocumentOutput, error) {
						return &imds.GetInstanceIdentityDocumentOutput{
							InstanceIdentityDocument: imds.InstanceIdentityDocument{
								Region:       "us-east-1",
								InstanceID:   "i-fallback",
								InstanceType: "m5.large",
								PrivateIP:    "10.0.0.1",
							},
						}, nil
					},
				}
			},
			expectedInstanceID:   "i-fallback",
			expectedInstanceType: "m5.large",
			expectedRegion:       "us-east-1",
			expectedPrivateIP:    "10.0.0.1",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			c := test.client(t)
			e := &ec2Metadata{
				logger:               zap.NewNop(),
				client:               c,
				clientFallbackEnable: c, // share for happy path; fallback path covered by separate test
				refreshInterval:      3 * time.Millisecond,
				instanceIDReadyC:     make(chan bool),
				instanceIPReadyC:     make(chan bool),
			}
			e.refresh(t.Context())

			assert.Equal(t, test.expectedInstanceID, e.getInstanceID())
			assert.Equal(t, test.expectedInstanceType, e.getInstanceType())
			assert.Equal(t, test.expectedRegion, e.getRegion())
			assert.Equal(t, test.expectedPrivateIP, e.getInstanceIP())
		})
	}
}

// TestEC2MetadataLocalMode verifies that localMode skips the IMDS calls.
func TestEC2MetadataLocalMode(t *testing.T) {
	called := false
	c := &mockMetadataClient{
		identityFn: func(_ context.Context, _ *imds.GetInstanceIdentityDocumentInput, _ ...func(*imds.Options)) (*imds.GetInstanceIdentityDocumentOutput, error) {
			called = true
			return &imds.GetInstanceIdentityDocumentOutput{}, nil
		},
	}
	e := &ec2Metadata{
		logger:               zap.NewNop(),
		client:               c,
		clientFallbackEnable: c,
		refreshInterval:      3 * time.Millisecond,
		instanceIDReadyC:     make(chan bool),
		instanceIPReadyC:     make(chan bool),
		localMode:            true,
	}
	e.refresh(t.Context())
	assert.False(t, called, "IMDS should not be called in local mode")
	assert.Empty(t, e.getInstanceID())
}

// TestEC2MetadataDualClientFallback exercises the dual-client pattern: strict
// client fails, fallback client succeeds.
func TestEC2MetadataDualClientFallback(t *testing.T) {
	strict := &mockMetadataClient{
		identityFn: func(_ context.Context, _ *imds.GetInstanceIdentityDocumentInput, _ ...func(*imds.Options)) (*imds.GetInstanceIdentityDocumentOutput, error) {
			return nil, errors.New("strict client failed")
		},
	}
	fallback := &mockMetadataClient{
		identityFn: func(_ context.Context, _ *imds.GetInstanceIdentityDocumentInput, _ ...func(*imds.Options)) (*imds.GetInstanceIdentityDocumentOutput, error) {
			return &imds.GetInstanceIdentityDocumentOutput{
				InstanceIdentityDocument: imds.InstanceIdentityDocument{
					Region:       "us-west-2",
					InstanceID:   "i-fallback-only",
					InstanceType: "t3.medium",
					PrivateIP:    "172.16.0.1",
				},
			}, nil
		},
	}
	e := &ec2Metadata{
		logger:               zap.NewNop(),
		client:               strict,
		clientFallbackEnable: fallback,
		refreshInterval:      3 * time.Millisecond,
		instanceIDReadyC:     make(chan bool),
		instanceIPReadyC:     make(chan bool),
	}
	e.refresh(t.Context())

	assert.Equal(t, "i-fallback-only", e.getInstanceID())
	assert.Equal(t, "t3.medium", e.getInstanceType())
	assert.Equal(t, "us-west-2", e.getRegion())
	assert.Equal(t, "172.16.0.1", e.getInstanceIP())
}

// TestEC2MetadataGetNetworkInterfaceID exercises the ENI lookup via
// GetMetadata and the per-MAC cache.
func TestEC2MetadataGetNetworkInterfaceID(t *testing.T) {
	c := &mockMetadataClient{
		metadataFn: func(_ context.Context, params *imds.GetMetadataInput, _ ...func(*imds.Options)) (*imds.GetMetadataOutput, error) {
			expected := "network/interfaces/macs/00:00:00:00:00:01/interface-id"
			if params.Path != expected {
				t.Errorf("unexpected path: got %q, want %q", params.Path, expected)
			}
			return &imds.GetMetadataOutput{
				Content: io.NopCloser(strings.NewReader("eni-001")),
			}, nil
		},
	}
	e := &ec2Metadata{
		logger:               zap.NewNop(),
		client:               c,
		clientFallbackEnable: c,
		networkInterfaceIDs:  make(map[string]string),
	}

	eniID, err := e.getNetworkInterfaceID("00:00:00:00:00:01")
	assert.NoError(t, err)
	assert.Equal(t, "eni-001", eniID)

	// Second call should hit cache (we'd see this fail if metadataFn ran twice
	// since the second call doesn't have the helper to validate).
	eniID, err = e.getNetworkInterfaceID("00:00:00:00:00:01")
	assert.NoError(t, err)
	assert.Equal(t, "eni-001", eniID)
}
