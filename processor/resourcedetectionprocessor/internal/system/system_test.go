// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package system

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/processor/processortest"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/metadataproviders/system"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/system/internal/metadata"
)

var _ system.Provider = (*mockMetadata)(nil)

type mockMetadata struct {
	mock.Mock
}

func (m *mockMetadata) Hostname() (string, error) {
	args := m.MethodCalled("Hostname")
	return args.String(0), args.Error(1)
}

func (m *mockMetadata) FQDN() (string, error) {
	args := m.MethodCalled("FQDN")
	return args.String(0), args.Error(1)
}

func (m *mockMetadata) OSType() (string, error) {
	args := m.MethodCalled("OSType")
	return args.String(0), args.Error(1)
}

func (m *mockMetadata) HostID(_ context.Context) (string, error) {
	args := m.MethodCalled("HostID")
	return args.String(0), args.Error(1)
}

func (m *mockMetadata) HostArch() (string, error) {
	args := m.MethodCalled("HostArch")
	return args.String(0), args.Error(1)
}

func (m *mockMetadata) LookupCNAME() (string, error) {
	args := m.MethodCalled("LookupCNAME")
	return args.String(0), args.Error(1)
}

func (m *mockMetadata) ReverseLookupHost() (string, error) {
	args := m.MethodCalled("ReverseLookupHost")
	return args.String(0), args.Error(1)
}

func TestNewDetector(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
	}{
		{
			name: "Success Case Valid Config 'HostnameSources' set to 'os'",
			cfg: Config{
				HostnameSources: []string{"os"},
			},
		},
		{
			name: "Success Case Valid Config 'HostnameSources' set to 'dns'",
			cfg: Config{
				HostnameSources: []string{"dns"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			detector, err := NewDetector(processortest.NewNopCreateSettings(), tt.cfg)
			assert.NotNil(t, detector)
			assert.NoError(t, err)
		})
	}
}

func allEnabledConfig() metadata.ResourceAttributesConfig {
	cfg := metadata.DefaultResourceAttributesConfig()
	cfg.HostArch.Enabled = true
	return cfg
}

func TestDetectFQDNAvailable(t *testing.T) {
	md := &mockMetadata{}
	md.On("FQDN").Return("fqdn", nil)
	md.On("OSType").Return("darwin", nil)
	md.On("HostID").Return("2", nil)
	md.On("HostArch").Return("amd64", nil)

	resourceAttributes := allEnabledConfig()
	detector := &Detector{provider: md, logger: zap.NewNop(), hostnameSources: []string{"dns"}, resourceAttributes: resourceAttributes}
	res, schemaURL, err := detector.Detect(context.Background())
	require.NoError(t, err)
	assert.Equal(t, conventions.SchemaURL, schemaURL)
	md.AssertExpectations(t)

	expected := map[string]any{
		conventions.AttributeHostName: "fqdn",
		conventions.AttributeOSType:   "darwin",
		conventions.AttributeHostID:   "2",
		conventions.AttributeHostArch: conventions.AttributeHostArchAMD64,
	}

	assert.Equal(t, expected, res.Attributes().AsRaw())

}

func TestFallbackHostname(t *testing.T) {
	mdHostname := &mockMetadata{}
	mdHostname.On("Hostname").Return("hostname", nil)
	mdHostname.On("FQDN").Return("", errors.New("err"))
	mdHostname.On("OSType").Return("darwin", nil)
	mdHostname.On("OSType").Return("darwin", nil)
	mdHostname.On("HostID").Return("3", nil)
	mdHostname.On("HostArch").Return("amd64", nil)

	resourceAttributes := allEnabledConfig()
	detector := &Detector{provider: mdHostname, logger: zap.NewNop(), hostnameSources: []string{"dns", "os"}, resourceAttributes: resourceAttributes}
	res, schemaURL, err := detector.Detect(context.Background())
	require.NoError(t, err)
	assert.Equal(t, conventions.SchemaURL, schemaURL)
	mdHostname.AssertExpectations(t)

	expected := map[string]any{
		conventions.AttributeHostName: "hostname",
		conventions.AttributeOSType:   "darwin",
		conventions.AttributeHostID:   "3",
		conventions.AttributeHostArch: conventions.AttributeHostArchAMD64,
	}

	assert.Equal(t, expected, res.Attributes().AsRaw())
}

func TestUseHostname(t *testing.T) {
	mdHostname := &mockMetadata{}
	mdHostname.On("Hostname").Return("hostname", nil)
	mdHostname.On("OSType").Return("darwin", nil)
	mdHostname.On("HostID").Return("1", nil)
	mdHostname.On("HostArch").Return("amd64", nil)

	resourceAttributes := allEnabledConfig()
	detector := &Detector{provider: mdHostname, logger: zap.NewNop(), hostnameSources: []string{"os"}, resourceAttributes: resourceAttributes}
	res, schemaURL, err := detector.Detect(context.Background())
	require.NoError(t, err)
	assert.Equal(t, conventions.SchemaURL, schemaURL)
	mdHostname.AssertExpectations(t)

	expected := map[string]any{
		conventions.AttributeHostName: "hostname",
		conventions.AttributeOSType:   "darwin",
		conventions.AttributeHostID:   "1",
		conventions.AttributeHostArch: conventions.AttributeHostArchAMD64,
	}

	assert.Equal(t, expected, res.Attributes().AsRaw())
}

func TestDetectError(t *testing.T) {
	// FQDN and hostname fail with 'hostnameSources' set to 'dns'
	mdFQDN := &mockMetadata{}
	mdFQDN.On("OSType").Return("windows", nil)
	mdFQDN.On("FQDN").Return("", errors.New("err"))
	mdFQDN.On("Hostname").Return("", errors.New("err"))
	mdFQDN.On("HostID").Return("", errors.New("err"))
	mdFQDN.On("HostArch").Return("amd64", nil)

	resourceAttributes := allEnabledConfig()
	detector := &Detector{provider: mdFQDN, logger: zap.NewNop(), hostnameSources: []string{"dns"}, resourceAttributes: resourceAttributes}
	res, schemaURL, err := detector.Detect(context.Background())
	assert.Error(t, err)
	assert.Equal(t, "", schemaURL)
	assert.True(t, internal.IsEmptyResource(res))

	// hostname fail with 'hostnameSources' set to 'os'
	mdHostname := &mockMetadata{}
	mdHostname.On("OSType").Return("windows", nil)
	mdHostname.On("Hostname").Return("", errors.New("err"))
	mdHostname.On("HostID").Return("", errors.New("err"))
	mdHostname.On("HostArch").Return("amd64", nil)

	detector = &Detector{provider: mdHostname, logger: zap.NewNop(), hostnameSources: []string{"os"}, resourceAttributes: resourceAttributes}
	res, schemaURL, err = detector.Detect(context.Background())
	assert.Error(t, err)
	assert.Equal(t, "", schemaURL)
	assert.True(t, internal.IsEmptyResource(res))

	// OS type fails
	mdOSType := &mockMetadata{}
	mdOSType.On("FQDN").Return("fqdn", nil)
	mdOSType.On("OSType").Return("", errors.New("err"))
	mdOSType.On("HostID").Return("", errors.New("err"))
	mdOSType.On("HostArch").Return("amd64", nil)

	detector = &Detector{provider: mdOSType, logger: zap.NewNop(), hostnameSources: []string{"dns"}, resourceAttributes: resourceAttributes}
	res, schemaURL, err = detector.Detect(context.Background())
	assert.Error(t, err)
	assert.Equal(t, "", schemaURL)
	assert.True(t, internal.IsEmptyResource(res))
}
