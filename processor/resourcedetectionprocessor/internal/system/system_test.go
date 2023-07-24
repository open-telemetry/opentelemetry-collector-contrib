// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package system

import (
	"context"
	"errors"
	"net"
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

func (m *mockMetadata) HostIPv4Addresses() ([]net.IP, error) {
	args := m.MethodCalled("HostIPv4Addresses")
	return args.Get(0).([]net.IP), args.Error(1)
}

func (m *mockMetadata) HostIPv6Addresses() ([]net.IP, error) {
	args := m.MethodCalled("HostIPv6Addresses")
	return args.Get(0).([]net.IP), args.Error(1)
}

var (
	testIPv4Attribute = []any{"192.168.1.140"}
	testIPv4Addresses = []net.IP{net.ParseIP(testIPv4Attribute[0].(string))}
	testIPv6Attribute = []any{"fe80::abc2:4a28:737a:609e", "fe80::849e:eaff:fe31:3c90"}
	testIPv6Addresses = []net.IP{net.ParseIP(testIPv6Attribute[0].(string)), net.ParseIP(testIPv6Attribute[1].(string))}
)

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
	cfg.HostID.Enabled = true
	cfg.HostIpv4Addresses.Enabled = true
	cfg.HostIpv6Addresses.Enabled = true
	return cfg
}

func TestDetectFQDNAvailable(t *testing.T) {
	md := &mockMetadata{}
	md.On("FQDN").Return("fqdn", nil)
	md.On("OSType").Return("darwin", nil)
	md.On("HostID").Return("2", nil)
	md.On("HostArch").Return("amd64", nil)
	md.On("HostIPv4Addresses").Return(testIPv4Addresses, nil)
	md.On("HostIPv6Addresses").Return(testIPv6Addresses, nil)

	detector := &Detector{provider: md, logger: zap.NewNop(), hostnameSources: []string{"dns"},
		rb: metadata.NewResourceBuilder(allEnabledConfig())}
	res, schemaURL, err := detector.Detect(context.Background())
	require.NoError(t, err)
	assert.Equal(t, conventions.SchemaURL, schemaURL)
	md.AssertExpectations(t)

	expected := map[string]any{
		conventions.AttributeHostName: "fqdn",
		conventions.AttributeOSType:   "darwin",
		conventions.AttributeHostID:   "2",
		conventions.AttributeHostArch: conventions.AttributeHostArchAMD64,
		"host.ipv4.addresses":         testIPv4Attribute,
		"host.ipv6.addresses":         testIPv6Attribute,
	}

	assert.Equal(t, expected, res.Attributes().AsRaw())

}

func TestFallbackHostname(t *testing.T) {
	mdHostname := &mockMetadata{}
	mdHostname.On("Hostname").Return("hostname", nil)
	mdHostname.On("FQDN").Return("", errors.New("err"))
	mdHostname.On("OSType").Return("darwin", nil)
	mdHostname.On("HostID").Return("3", nil)
	mdHostname.On("HostArch").Return("amd64", nil)
	mdHostname.On("HostIPv4Addresses").Return(testIPv4Addresses, nil)
	mdHostname.On("HostIPv6Addresses").Return(testIPv6Addresses, nil)

	detector := &Detector{provider: mdHostname, logger: zap.NewNop(), hostnameSources: []string{"dns", "os"},
		rb: metadata.NewResourceBuilder(metadata.DefaultResourceAttributesConfig())}
	res, schemaURL, err := detector.Detect(context.Background())
	require.NoError(t, err)
	assert.Equal(t, conventions.SchemaURL, schemaURL)
	mdHostname.AssertExpectations(t)

	expected := map[string]any{
		conventions.AttributeHostName: "hostname",
		conventions.AttributeOSType:   "darwin",
	}

	assert.Equal(t, expected, res.Attributes().AsRaw())
}

func TestEnableHostID(t *testing.T) {
	mdHostname := &mockMetadata{}
	mdHostname.On("Hostname").Return("hostname", nil)
	mdHostname.On("FQDN").Return("", errors.New("err"))
	mdHostname.On("OSType").Return("darwin", nil)
	mdHostname.On("HostID").Return("3", nil)
	mdHostname.On("HostArch").Return("amd64", nil)
	mdHostname.On("HostIPv4Addresses").Return(testIPv4Addresses, nil)
	mdHostname.On("HostIPv6Addresses").Return(testIPv6Addresses, nil)

	detector := &Detector{provider: mdHostname, logger: zap.NewNop(), hostnameSources: []string{"dns", "os"},
		rb: metadata.NewResourceBuilder(allEnabledConfig())}
	res, schemaURL, err := detector.Detect(context.Background())
	require.NoError(t, err)
	assert.Equal(t, conventions.SchemaURL, schemaURL)
	mdHostname.AssertExpectations(t)

	expected := map[string]any{
		conventions.AttributeHostName: "hostname",
		conventions.AttributeOSType:   "darwin",
		conventions.AttributeHostID:   "3",
		conventions.AttributeHostArch: conventions.AttributeHostArchAMD64,
		"host.ipv4.addresses":         testIPv4Attribute,
		"host.ipv6.addresses":         testIPv6Attribute,
	}

	assert.Equal(t, expected, res.Attributes().AsRaw())
}

func TestUseHostname(t *testing.T) {
	mdHostname := &mockMetadata{}
	mdHostname.On("Hostname").Return("hostname", nil)
	mdHostname.On("OSType").Return("darwin", nil)
	mdHostname.On("HostID").Return("1", nil)
	mdHostname.On("HostArch").Return("amd64", nil)
	mdHostname.On("HostIPv4Addresses").Return(testIPv4Addresses, nil)
	mdHostname.On("HostIPv6Addresses").Return(testIPv6Addresses, nil)

	detector := &Detector{provider: mdHostname, logger: zap.NewNop(), hostnameSources: []string{"os"},
		rb: metadata.NewResourceBuilder(allEnabledConfig())}
	res, schemaURL, err := detector.Detect(context.Background())
	require.NoError(t, err)
	assert.Equal(t, conventions.SchemaURL, schemaURL)
	mdHostname.AssertExpectations(t)

	expected := map[string]any{
		conventions.AttributeHostName: "hostname",
		conventions.AttributeOSType:   "darwin",
		conventions.AttributeHostID:   "1",
		conventions.AttributeHostArch: conventions.AttributeHostArchAMD64,
		"host.ipv4.addresses":         testIPv4Attribute,
		"host.ipv6.addresses":         testIPv6Attribute,
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
	mdFQDN.On("HostIPv4Addresses").Return(testIPv4Addresses, nil)
	mdFQDN.On("HostIPv6Addresses").Return(testIPv6Addresses, nil)

	detector := &Detector{provider: mdFQDN, logger: zap.NewNop(), hostnameSources: []string{"dns"},
		rb: metadata.NewResourceBuilder(allEnabledConfig())}
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
	mdHostname.On("HostIPv4Addresses").Return(testIPv4Addresses, nil)
	mdHostname.On("HostIPv6Addresses").Return(testIPv6Addresses, nil)

	detector = &Detector{provider: mdHostname, logger: zap.NewNop(), hostnameSources: []string{"os"},
		rb: metadata.NewResourceBuilder(allEnabledConfig())}
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
	mdOSType.On("HostIPv4Addresses").Return(testIPv4Addresses, nil)
	mdOSType.On("HostIPv6Addresses").Return(testIPv6Addresses, nil)

	detector = &Detector{provider: mdOSType, logger: zap.NewNop(), hostnameSources: []string{"dns"},
		rb: metadata.NewResourceBuilder(allEnabledConfig())}
	res, schemaURL, err = detector.Detect(context.Background())
	assert.Error(t, err)
	assert.Equal(t, "", schemaURL)
	assert.True(t, internal.IsEmptyResource(res))
}
