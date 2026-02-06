// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cvm

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/processor/processortest"
	"go.uber.org/zap"

	cvmprovider "github.com/open-telemetry/opentelemetry-collector-contrib/internal/metadataproviders/tencent/cvm"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/tencent/cvm/internal/metadata"
)

type mockCVMMetadata struct {
	retMetadata *cvmprovider.Metadata
	retErr      error
}

var _ cvmprovider.Provider = (*mockCVMMetadata)(nil)

func (m *mockCVMMetadata) Metadata(_ context.Context) (*cvmprovider.Metadata, error) {
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
		provider              cvmprovider.Provider
		failOnMissingMetadata bool
		want                  pcommon.Resource
		wantErr               bool
	}{
		{
			name: "success with all metadata",
			provider: &mockCVMMetadata{
				retMetadata: &cvmprovider.Metadata{
					InstanceName: "cvm-test-instance",
					ImageID:      "img-abcdef123456",
					InstanceID:   "ins-abcdef123456",
					InstanceType: "S5.MEDIUM2",
					AppID:        "1234567890",
					RegionID:     "ap-guangzhou",
					ZoneID:       "ap-guangzhou-3",
				},
			},
			failOnMissingMetadata: false,
			want: func() pcommon.Resource {
				res := pcommon.NewResource()
				attr := res.Attributes()
				attr.PutStr("cloud.provider", "tencent_cloud")
				attr.PutStr("cloud.platform", "tencent_cloud_cvm")
				attr.PutStr("cloud.account.id", "1234567890")
				attr.PutStr("cloud.region", "ap-guangzhou")
				attr.PutStr("cloud.availability_zone", "ap-guangzhou-3")
				attr.PutStr("host.id", "ins-abcdef123456")
				attr.PutStr("host.name", "cvm-test-instance")
				attr.PutStr("host.image.id", "img-abcdef123456")
				attr.PutStr("host.type", "S5.MEDIUM2")
				return res
			}(),
		},
		{
			name: "metadata unavailable - fail_on_missing_metadata false",
			provider: &mockCVMMetadata{
				retErr: errors.New("metadata unavailable"),
			},
			failOnMissingMetadata: false,
			want:                  pcommon.NewResource(),
			wantErr:               false,
		},
		{
			name: "metadata unavailable - fail_on_missing_metadata true",
			provider: &mockCVMMetadata{
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
