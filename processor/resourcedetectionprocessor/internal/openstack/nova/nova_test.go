// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package nova

import (
	"context"
	"errors"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/processor/processortest"
	"go.uber.org/zap"

	novaprovider "github.com/open-telemetry/opentelemetry-collector-contrib/internal/metadataproviders/openstack/nova"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/openstack/nova/internal/metadata"
)

var errUnavailable = errors.New("nova metadata unavailable")

// --- Provider mock ---

type mockNovaMetadata struct {
	retDoc             novaprovider.Document
	retErrGet          error
	retHostname        string
	retErrHostname     error
	retInstanceType    string
	retErrInstanceType error
	isAvailable        bool
}

var _ novaprovider.Provider = (*mockNovaMetadata)(nil)

func (m *mockNovaMetadata) InstanceID(_ context.Context) (string, error) {
	if !m.isAvailable {
		return "", errUnavailable
	}
	return m.retDoc.UUID, nil
}

func (m *mockNovaMetadata) Get(_ context.Context) (novaprovider.Document, error) {
	if m.retErrGet != nil {
		return novaprovider.Document{}, m.retErrGet
	}
	return m.retDoc, nil
}

func (m *mockNovaMetadata) Hostname(_ context.Context) (string, error) {
	if m.retErrHostname != nil {
		return "", m.retErrHostname
	}
	return m.retHostname, nil
}

func (m *mockNovaMetadata) InstanceType(_ context.Context) (string, error) {
	if m.retErrInstanceType != nil {
		return "", m.retErrInstanceType
	}
	if m.retInstanceType != "" {
		return m.retInstanceType, nil
	}
	return "dummy.host.type", nil
}

// --- Constructor smoke test (matches EC2 structure) ---

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
			name: "Success Case Valid Config",
			cfg: Config{
				Labels: []string{"label1"},
			},
			shouldError: false,
		},
		{
			name: "Error Case Invalid Regex",
			cfg: Config{
				Labels: []string{"*"},
			},
			shouldError: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			detector, err := NewDetector(processortest.NewNopSettings(processortest.NopType), tt.cfg)
			if tt.shouldError {
				assert.Error(t, err)
				assert.Nil(t, detector)
			} else {
				assert.NotNil(t, detector)
				assert.NoError(t, err)
			}
		})
	}
}

// --- Detect() behavior tests (like EC2’s TestDetector_Detect) ---

func TestDetector_Detect(t *testing.T) {
	tests := []struct {
		name                  string
		provider              novaprovider.Provider
		labelRegexes          []*regexp.Regexp
		want                  pcommon.Resource
		wantErr               bool
		failOnMissingMetadata bool
	}{
		{
			name: "success with availability_zone, project_id and labels",
			provider: &mockNovaMetadata{
				retDoc: novaprovider.Document{
					UUID:             "vm-1234",
					ProjectID:        "proj-xyz",
					AvailabilityZone: "zone-a",
					Meta: map[string]string{
						"tag1":  "val1",
						"tag2":  "val2",
						"other": "nope",
					},
				},
				retHostname:     "example-nova-host",
				retInstanceType: "dummy.host.type",
				isAvailable:     true,
			},
			labelRegexes: []*regexp.Regexp{regexp.MustCompile("^tag1$"), regexp.MustCompile("^tag2$")},
			want: func() pcommon.Resource {
				res := pcommon.NewResource()
				attr := res.Attributes()
				attr.PutStr("cloud.provider", "openstack")
				attr.PutStr("cloud.platform", "openstack_nova")
				attr.PutStr("cloud.account.id", "proj-xyz")
				attr.PutStr("cloud.availability_zone", "zone-a")
				attr.PutStr("host.id", "vm-1234")
				attr.PutStr("host.name", "example-nova-host")
				attr.PutStr("host.type", "dummy.host.type")
				attr.PutStr("openstack.nova.meta.tag1", "val1")
				attr.PutStr("openstack.nova.meta.tag2", "val2")
				return res
			}(),
		},
		{
			name: "endpoint not available",
			provider: &mockNovaMetadata{
				isAvailable: false,
			},
			want:    pcommon.NewResource(),
			wantErr: false,
		},
		{
			name: "endpoint not available, fail_on_missing_metadata",
			provider: &mockNovaMetadata{
				isAvailable: false,
			},
			want:                  pcommon.NewResource(),
			wantErr:               true,
			failOnMissingMetadata: true,
		},
		{
			name: "get fails",
			provider: &mockNovaMetadata{
				isAvailable: true,
				retErrGet:   errors.New("get failed"),
			},
			want:    pcommon.NewResource(),
			wantErr: true,
		},
		{
			name: "hostname fails",
			provider: &mockNovaMetadata{
				isAvailable:    true,
				retErrHostname: errors.New("hostname failed"),
			},
			want:    pcommon.NewResource(),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &Detector{
				metadataProvider:      tt.provider,
				logger:                zap.NewNop(),
				rb:                    metadata.NewResourceBuilder(metadata.DefaultResourceAttributesConfig()),
				labelRegexes:          tt.labelRegexes,
				failOnMissingMetadata: tt.failOnMissingMetadata,
			}

			got, _, err := d.Detect(t.Context())
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.NotNil(t, got)
				assert.Equal(t, tt.want.Attributes().AsRaw(), got.Attributes().AsRaw())
			}
		})
	}
}
