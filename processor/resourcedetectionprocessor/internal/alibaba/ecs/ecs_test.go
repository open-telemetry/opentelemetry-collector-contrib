// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ecs

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/processor/processortest"
	"go.uber.org/zap"

	ecsprovider "github.com/open-telemetry/opentelemetry-collector-contrib/internal/metadataproviders/alibaba/ecs"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/alibaba/ecs/internal/metadata"
)

type mockECSMetadata struct {
	retMetadata *ecsprovider.Metadata
	retErr      error
}

var _ ecsprovider.Provider = (*mockECSMetadata)(nil)

func (m *mockECSMetadata) Metadata(_ context.Context) (*ecsprovider.Metadata, error) {
	if m.retErr != nil {
		return nil, m.retErr
	}
	return m.retMetadata, nil
}

// --- Constructor smoke test ---

func TestNewDetector(t *testing.T) {
	tests := []struct {
		name        string
		cfg         Config
		shouldError bool
	}{
		{
			name:        "Success Case Empty Config",
			cfg:         Config{},
			shouldError: false,
		},
		{
			name:        "Success Case Default Config",
			cfg:         CreateDefaultConfig(),
			shouldError: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			detector, err := NewDetector(processortest.NewNopSettings(processortest.NopType), tt.cfg)
			if tt.shouldError {
				require.Error(t, err)
				require.Nil(t, detector)
			} else {
				require.NotNil(t, detector)
				require.NoError(t, err)
			}
		})
	}
}

func TestDetector_Detect(t *testing.T) {
	tests := []struct {
		name                  string
		provider              ecsprovider.Provider
		failOnMissingMetadata bool
		want                  pcommon.Resource
		wantErr               bool
	}{
		{
			name: "success with all metadata",
			provider: &mockECSMetadata{
				retMetadata: &ecsprovider.Metadata{
					Hostname:       "ecs-instance-01",
					ImageID:        "m-abcdef123456",
					InstanceID:     "i-abcdef123456",
					InstanceType:   "ecs.g6.large",
					OwnerAccountID: "1234567890123456",
					RegionID:       "cn-hangzhou",
					ZoneID:         "cn-hangzhou-a",
				},
			},
			failOnMissingMetadata: false,
			want: func() pcommon.Resource {
				res := pcommon.NewResource()
				attr := res.Attributes()
				attr.PutStr("cloud.provider", "alibaba_cloud")
				attr.PutStr("cloud.platform", "alibaba_cloud_ecs")
				attr.PutStr("cloud.account.id", "1234567890123456")
				attr.PutStr("cloud.region", "cn-hangzhou")
				attr.PutStr("cloud.availability_zone", "cn-hangzhou-a")
				attr.PutStr("host.id", "i-abcdef123456")
				attr.PutStr("host.name", "ecs-instance-01")
				attr.PutStr("host.image.id", "m-abcdef123456")
				attr.PutStr("host.type", "ecs.g6.large")
				return res
			}(),
		},
		{
			name: "metadata unavailable - fail_on_missing_metadata false",
			provider: &mockECSMetadata{
				retErr: errors.New("metadata unavailable"),
			},
			failOnMissingMetadata: false,
			want:                  pcommon.NewResource(),
			wantErr:               false,
		},
		{
			name: "metadata unavailable - fail_on_missing_metadata true",
			provider: &mockECSMetadata{
				retErr: errors.New("metadata unavailable"),
			},
			failOnMissingMetadata: true,
			want:                  pcommon.NewResource(),
			wantErr:               true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &Detector{
				metadataProvider:      tt.provider,
				logger:                zap.NewNop(),
				rb:                    metadata.NewResourceBuilder(metadata.DefaultResourceAttributesConfig()),
				failOnMissingMetadata: tt.failOnMissingMetadata,
			}

			got, _, err := d.Detect(t.Context())
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.NotNil(t, got)
				require.Equal(t, tt.want.Attributes().AsRaw(), got.Attributes().AsRaw())
			}
		})
	}
}
