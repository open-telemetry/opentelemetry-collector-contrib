// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package system

import (
	"context"
	"errors"
	"fmt"
	"net"
	"testing"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/processor/processortest"
	conventions "go.opentelemetry.io/otel/semconv/v1.6.1"
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

func (m *mockMetadata) OSDescription(_ context.Context) (string, error) {
	args := m.MethodCalled("OSDescription")
	return args.String(0), args.Error(1)
}

func (m *mockMetadata) OSType() (string, error) {
	args := m.MethodCalled("OSType")
	return args.String(0), args.Error(1)
}

func (m *mockMetadata) OSVersion() (string, error) {
	args := m.MethodCalled("OSVersion")
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

func (m *mockMetadata) HostIPs() ([]net.IP, error) {
	args := m.MethodCalled("HostIPs")
	return args.Get(0).([]net.IP), args.Error(1)
}

func (m *mockMetadata) HostMACs() ([]net.HardwareAddr, error) {
	args := m.MethodCalled("HostMACs")
	return args.Get(0).([]net.HardwareAddr), args.Error(1)
}

func (m *mockMetadata) HostInterfaces() ([]net.Interface, error) {
	args := m.MethodCalled("HostInterfaces")
	return args.Get(0).([]net.Interface), args.Error(1)
}

func (m *mockMetadata) CPUInfo(_ context.Context) ([]cpu.InfoStat, error) {
	args := m.MethodCalled("CPUInfo")
	return args.Get(0).([]cpu.InfoStat), args.Error(1)
}

// OSName returns a mock OS name.
func (m *mockMetadata) OSName(_ context.Context) (string, error) {
	args := m.MethodCalled("OSName")
	return args.String(0), args.Error(1)
}

// OSBuildID returns a mock OS build ID.
func (m *mockMetadata) OSBuildID(_ context.Context) (string, error) {
	args := m.MethodCalled("OSBuildID")
	return args.String(0), args.Error(1)
}

var (
	testIPsAttribute = []any{"192.168.1.140", "fe80::abc2:4a28:737a:609e"}
	testIPsAddresses = []net.IP{net.ParseIP(testIPsAttribute[0].(string)), net.ParseIP(testIPsAttribute[1].(string))}

	testMACsAttribute = []any{"00-00-00-00-00-01", "DE-AD-BE-EF-00-00"}
	testMACsAddresses = []net.HardwareAddr{{0x00, 0x00, 0x00, 0x00, 0x00, 0x01}, {0xDE, 0xAD, 0xBE, 0xEF, 0x00, 0x00}}

	testInterfacesAttribute = []any{"eth0", "wlan0"}
	testInterfaces          = []net.Interface{
		{
			Index:        1,
			MTU:          1500,
			Name:         "eth0",
			HardwareAddr: net.HardwareAddr{0x00, 0x0c, 0x29, 0xaa, 0xbb, 0xcc},
			Flags:        net.FlagUp | net.FlagBroadcast | net.FlagMulticast,
		},
		{
			Index:        2,
			MTU:          1500,
			Name:         "wlan0",
			HardwareAddr: net.HardwareAddr{0x00, 0x0c, 0x29, 0xdd, 0xee, 0xff},
			Flags:        net.FlagUp | net.FlagBroadcast | net.FlagMulticast,
		},
	}
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
			detector, err := NewDetector(processortest.NewNopSettings(processortest.NopType), tt.cfg)
			assert.NotNil(t, detector)
			assert.NoError(t, err)
		})
	}
}

func TestToIEEERA(t *testing.T) {
	tests := []struct {
		addr     net.HardwareAddr
		expected string
	}{
		{
			addr:     testMACsAddresses[0],
			expected: testMACsAttribute[0].(string),
		},
		{
			addr:     testMACsAddresses[1],
			expected: testMACsAttribute[1].(string),
		},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, toIEEERA(tt.addr))
		})
	}
}

func allEnabledConfig() metadata.ResourceAttributesConfig {
	cfg := metadata.DefaultResourceAttributesConfig()
	cfg.HostArch.Enabled = true
	cfg.HostID.Enabled = true
	cfg.HostIP.Enabled = true
	cfg.HostMac.Enabled = true
	cfg.HostInterface.Enabled = true
	cfg.OsDescription.Enabled = true
	cfg.OsVersion.Enabled = true
	return cfg
}

func TestDetectFQDNAvailable(t *testing.T) {
	md := &mockMetadata{}
	md.On("FQDN").Return("fqdn", nil)
	md.On("OSDescription").Return("Ubuntu 22.04.2 LTS (Jammy Jellyfish)", nil)
	md.On("OSType").Return("darwin", nil)
	md.On("OSVersion").Return("22.04.2 LTS (Jammy Jellyfish)", nil)
	md.On("HostID").Return("2", nil)
	md.On("HostArch").Return("amd64", nil)
	md.On("HostIPs").Return(testIPsAddresses, nil)
	md.On("HostMACs").Return(testMACsAddresses, nil)
	md.On("HostInterfaces").Return(testInterfaces, nil)

	detector := newTestDetector(md, []string{"dns"}, allEnabledConfig())
	res, schemaURL, err := detector.Detect(context.Background())
	require.NoError(t, err)
	assert.Equal(t, conventions.SchemaURL, schemaURL)
	md.AssertExpectations(t)
	md.AssertNotCalled(t, "CPUInfo")

	expected := map[string]any{
		string(conventions.HostNameKey):      "fqdn",
		string(conventions.OSDescriptionKey): "Ubuntu 22.04.2 LTS (Jammy Jellyfish)",
		string(conventions.OSTypeKey):        "darwin",
		string(conventions.OSVersionKey):     "22.04.2 LTS (Jammy Jellyfish)",
		string(conventions.HostIDKey):        "2",
		string(conventions.HostArchKey):      conventions.HostArchAMD64.Value.AsString(),
		"host.ip":                            testIPsAttribute,
		"host.mac":                           testMACsAttribute,
		"host.interface":                     testInterfacesAttribute,
	}

	assert.Equal(t, expected, res.Attributes().AsRaw())
}

func TestFallbackHostname(t *testing.T) {
	mdHostname := &mockMetadata{}
	mdHostname.On("Hostname").Return("hostname", nil)
	mdHostname.On("FQDN").Return("", errors.New("err"))
	mdHostname.On("OSDescription").Return("Ubuntu 22.04.2 LTS (Jammy Jellyfish)", nil)
	mdHostname.On("OSType").Return("darwin", nil)
	mdHostname.On("OSVersion").Return("22.04.2 LTS (Jammy Jellyfish)", nil)
	mdHostname.On("HostArch").Return("amd64", nil)

	detector := newTestDetector(mdHostname, []string{"dns", "os"}, metadata.DefaultResourceAttributesConfig())
	res, schemaURL, err := detector.Detect(context.Background())
	require.NoError(t, err)
	assert.Equal(t, conventions.SchemaURL, schemaURL)
	mdHostname.AssertExpectations(t)
	mdHostname.AssertNotCalled(t, "HostID")
	mdHostname.AssertNotCalled(t, "HostIPs")

	expected := map[string]any{
		string(conventions.HostNameKey): "hostname",
		string(conventions.OSTypeKey):   "darwin",
	}

	assert.Equal(t, expected, res.Attributes().AsRaw())
}

func TestEnableHostID(t *testing.T) {
	mdHostname := &mockMetadata{}
	mdHostname.On("Hostname").Return("hostname", nil)
	mdHostname.On("FQDN").Return("", errors.New("err"))
	mdHostname.On("OSDescription").Return("Ubuntu 22.04.2 LTS (Jammy Jellyfish)", nil)
	mdHostname.On("OSType").Return("darwin", nil)
	mdHostname.On("OSVersion").Return("22.04.2 LTS (Jammy Jellyfish)", nil)
	mdHostname.On("HostID").Return("3", nil)
	mdHostname.On("HostArch").Return("amd64", nil)
	mdHostname.On("HostIPs").Return(testIPsAddresses, nil)
	mdHostname.On("HostMACs").Return(testMACsAddresses, nil)
	mdHostname.On("HostInterfaces").Return(testInterfaces, nil)

	detector := newTestDetector(mdHostname, []string{"dns", "os"}, allEnabledConfig())
	res, schemaURL, err := detector.Detect(context.Background())
	require.NoError(t, err)
	assert.Equal(t, conventions.SchemaURL, schemaURL)
	mdHostname.AssertExpectations(t)

	expected := map[string]any{
		string(conventions.HostNameKey):      "hostname",
		string(conventions.OSDescriptionKey): "Ubuntu 22.04.2 LTS (Jammy Jellyfish)",
		string(conventions.OSTypeKey):        "darwin",
		string(conventions.OSVersionKey):     "22.04.2 LTS (Jammy Jellyfish)",
		string(conventions.HostIDKey):        "3",
		string(conventions.HostArchKey):      conventions.HostArchAMD64.Value.AsString(),
		"host.ip":                            testIPsAttribute,
		"host.mac":                           testMACsAttribute,
		"host.interface":                     testInterfacesAttribute,
	}

	assert.Equal(t, expected, res.Attributes().AsRaw())
}

func TestUseHostname(t *testing.T) {
	mdHostname := &mockMetadata{}
	mdHostname.On("Hostname").Return("hostname", nil)
	mdHostname.On("OSDescription").Return("Ubuntu 22.04.2 LTS (Jammy Jellyfish)", nil)
	mdHostname.On("OSType").Return("darwin", nil)
	mdHostname.On("OSVersion").Return("22.04.2 LTS (Jammy Jellyfish)", nil)
	mdHostname.On("HostID").Return("1", nil)
	mdHostname.On("HostArch").Return("amd64", nil)
	mdHostname.On("HostIPs").Return(testIPsAddresses, nil)
	mdHostname.On("HostMACs").Return(testMACsAddresses, nil)
	mdHostname.On("HostInterfaces").Return(testInterfaces, nil)

	detector := newTestDetector(mdHostname, []string{"os"}, allEnabledConfig())
	res, schemaURL, err := detector.Detect(context.Background())
	require.NoError(t, err)
	assert.Equal(t, conventions.SchemaURL, schemaURL)
	mdHostname.AssertExpectations(t)

	expected := map[string]any{
		string(conventions.HostNameKey):      "hostname",
		string(conventions.OSDescriptionKey): "Ubuntu 22.04.2 LTS (Jammy Jellyfish)",
		string(conventions.OSTypeKey):        "darwin",
		string(conventions.OSVersionKey):     "22.04.2 LTS (Jammy Jellyfish)",
		string(conventions.HostIDKey):        "1",
		string(conventions.HostArchKey):      conventions.HostArchAMD64.Value.AsString(),
		"host.ip":                            testIPsAttribute,
		"host.mac":                           testMACsAttribute,
		"host.interface":                     testInterfacesAttribute,
	}

	assert.Equal(t, expected, res.Attributes().AsRaw())
}

func TestDetectError(t *testing.T) {
	// FQDN and hostname fail with 'hostnameSources' set to 'dns'
	mdFQDN := &mockMetadata{}
	mdFQDN.On("OSDescription").Return("Ubuntu 22.04.2 LTS (Jammy Jellyfish)", nil)
	mdFQDN.On("OSType").Return("windows", nil)
	mdFQDN.On("OSVersion").Return("22.04.2 LTS (Jammy Jellyfish)", nil)
	mdFQDN.On("FQDN").Return("", errors.New("err"))
	mdFQDN.On("Hostname").Return("", errors.New("err"))
	mdFQDN.On("HostID").Return("", errors.New("err"))
	mdFQDN.On("HostArch").Return("amd64", nil)
	mdFQDN.On("HostIPs").Return(testIPsAddresses, nil)
	mdFQDN.On("HostMACs").Return(testMACsAddresses, nil)
	mdFQDN.On("HostInterfaces").Return(testInterfaces, nil)

	detector := newTestDetector(mdFQDN, []string{"dns"}, allEnabledConfig())
	res, schemaURL, err := detector.Detect(context.Background())
	assert.Error(t, err)
	assert.Empty(t, schemaURL)
	assert.True(t, internal.IsEmptyResource(res))

	// hostname fail with 'hostnameSources' set to 'os'
	mdHostname := &mockMetadata{}
	mdHostname.On("OSDescription").Return("Ubuntu 22.04.2 LTS (Jammy Jellyfish)", nil)
	mdHostname.On("OSType").Return("windows", nil)
	mdHostname.On("OSVersion").Return("22.04.2 LTS (Jammy Jellyfish)", nil)
	mdHostname.On("Hostname").Return("", errors.New("err"))
	mdHostname.On("HostID").Return("", errors.New("err"))
	mdHostname.On("HostArch").Return("amd64", nil)
	mdHostname.On("HostIPs").Return(testIPsAddresses, nil)
	mdHostname.On("HostMACs").Return(testMACsAddresses, nil)
	mdHostname.On("HostInterfaces").Return(testInterfaces, nil)

	detector = newTestDetector(mdHostname, []string{"os"}, allEnabledConfig())
	res, schemaURL, err = detector.Detect(context.Background())
	assert.Error(t, err)
	assert.Empty(t, schemaURL)
	assert.True(t, internal.IsEmptyResource(res))

	// OS type fails
	mdOSType := &mockMetadata{}
	mdOSType.On("FQDN").Return("fqdn", nil)
	mdOSType.On("OSDescription").Return("Ubuntu 22.04.2 LTS (Jammy Jellyfish)", nil)
	mdOSType.On("OSType").Return("", errors.New("err"))
	mdOSType.On("OSVersion").Return("", "22.04.2 LTS (Jammy Jellyfish)")
	mdOSType.On("HostID").Return("1", nil)
	mdOSType.On("HostArch").Return("amd64", nil)
	mdOSType.On("HostIPs").Return(testIPsAddresses, nil)
	mdOSType.On("HostInterfaces").Return(testInterfaces, nil)

	detector = newTestDetector(mdOSType, []string{"os"}, allEnabledConfig())
	res, schemaURL, err = detector.Detect(context.Background())
	assert.Error(t, err)
	assert.Empty(t, schemaURL)
	assert.True(t, internal.IsEmptyResource(res))

	// OS version fails
	mdOSVersion := &mockMetadata{}
	mdOSVersion.On("FQDN").Return("fqdn", nil)
	mdOSVersion.On("OSDescription").Return("Ubuntu 22.04.2 LTS (Jammy Jellyfish)", nil)
	mdOSVersion.On("OSType").Return("windows", nil)
	mdOSVersion.On("OSVersion").Return("", errors.New("err"))
	mdOSVersion.On("HostID").Return("1", nil)
	mdOSVersion.On("HostArch").Return("amd64", nil)
	mdOSVersion.On("HostIPs").Return(testIPsAddresses, nil)
	mdOSVersion.On("HostInterfaces").Return(testInterfaces, nil)

	detector = newTestDetector(mdOSVersion, []string{"os"}, allEnabledConfig())
	res, schemaURL, err = detector.Detect(context.Background())
	assert.Error(t, err)
	assert.Empty(t, schemaURL)
	assert.True(t, internal.IsEmptyResource(res))

	// Host ID fails. All other attributes should be set.
	mdHostID := &mockMetadata{}
	mdHostID.On("Hostname").Return("hostname", nil)
	mdHostID.On("OSDescription").Return("Ubuntu 22.04.2 LTS (Jammy Jellyfish)", nil)
	mdHostID.On("OSType").Return("linux", nil)
	mdHostID.On("OSVersion").Return("22.04.2 LTS (Jammy Jellyfish)", nil)
	mdHostID.On("HostID").Return("", errors.New("err"))
	mdHostID.On("HostArch").Return("arm64", nil)
	mdHostID.On("HostIPs").Return(testIPsAddresses, nil)
	mdHostID.On("HostMACs").Return(testMACsAddresses, nil)
	mdHostID.On("HostInterfaces").Return(testInterfaces, nil)

	detector = newTestDetector(mdHostID, []string{"os"}, allEnabledConfig())
	res, schemaURL, err = detector.Detect(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, conventions.SchemaURL, schemaURL)
	assert.Equal(t, map[string]any{
		string(conventions.HostNameKey):      "hostname",
		string(conventions.OSDescriptionKey): "Ubuntu 22.04.2 LTS (Jammy Jellyfish)",
		string(conventions.OSTypeKey):        "linux",
		string(conventions.OSVersionKey):     "22.04.2 LTS (Jammy Jellyfish)",
		string(conventions.HostArchKey):      conventions.HostArchARM64.Value.AsString(),
		"host.ip":                            testIPsAttribute,
		"host.mac":                           testMACsAttribute,
		"host.interface":                     testInterfacesAttribute,
	}, res.Attributes().AsRaw())
}

func TestDetectCPUInfo(t *testing.T) {
	md := &mockMetadata{}
	md.On("FQDN").Return("fqdn", nil)
	md.On("OSDescription").Return("Ubuntu 22.04.2 LTS (Jammy Jellyfish)", nil)
	md.On("OSType").Return("darwin", nil)
	md.On("OSVersion").Return("22.04.2 LTS (Jammy Jellyfish)", nil)
	md.On("HostID").Return("2", nil)
	md.On("HostArch").Return("amd64", nil)
	md.On("HostIPs").Return(testIPsAddresses, nil)
	md.On("HostMACs").Return(testMACsAddresses, nil)
	md.On("HostInterfaces").Return(testInterfaces, nil)
	md.On("CPUInfo").Return([]cpu.InfoStat{{Family: "some"}}, nil)

	cfg := allEnabledConfig()
	cfg.HostCPUFamily.Enabled = true
	detector := newTestDetector(md, []string{"dns"}, cfg)
	res, schemaURL, err := detector.Detect(context.Background())
	require.NoError(t, err)
	assert.Equal(t, conventions.SchemaURL, schemaURL)
	md.AssertExpectations(t)

	expected := map[string]any{
		string(conventions.HostNameKey):      "fqdn",
		string(conventions.OSDescriptionKey): "Ubuntu 22.04.2 LTS (Jammy Jellyfish)",
		string(conventions.OSTypeKey):        "darwin",
		string(conventions.OSVersionKey):     "22.04.2 LTS (Jammy Jellyfish)",
		string(conventions.HostIDKey):        "2",
		string(conventions.HostArchKey):      conventions.HostArchAMD64.Value.AsString(),
		"host.ip":                            testIPsAttribute,
		"host.mac":                           testMACsAttribute,
		"host.cpu.family":                    "some",
		"host.interface":                     testInterfacesAttribute,
	}

	assert.Equal(t, expected, res.Attributes().AsRaw())
}

func TestDetectOSNameAndBuildID(t *testing.T) {
	md := &mockMetadata{}
	md.On("FQDN").Return("fqdn", nil)
	md.On("OSDescription").Return("desc", nil)
	md.On("OSType").Return("type", nil)
	md.On("OSVersion").Return("ver", nil)
	md.On("OSName").Return("MyOS", nil)
	md.On("OSBuildID").Return("Build123", nil)
	md.On("HostArch").Return("amd64", nil)

	cfg := metadata.DefaultResourceAttributesConfig()
	cfg.OsName.Enabled = true
	cfg.OsBuildID.Enabled = true
	detector := newTestDetector(md, []string{"dns"}, cfg)
	res, _, err := detector.Detect(context.Background())
	require.NoError(t, err)
	attrs := res.Attributes().AsRaw()
	assert.Equal(t, "MyOS", attrs["os.name"])
	assert.Equal(t, "Build123", attrs["os.build.id"])
	md.AssertExpectations(t)
}

func TestHostInterfaces(t *testing.T) {
	mdInterfaces := &mockMetadata{}
	mdInterfaces.On("Hostname").Return("hostname", nil)
	mdInterfaces.On("OSDescription").Return("Ubuntu 22.04.2 LTS (Jammy Jellyfish)", nil)
	mdInterfaces.On("OSType").Return("linux", nil)
	mdInterfaces.On("OSVersion").Return("22.04.2 LTS (Jammy Jellyfish)", nil)
	mdInterfaces.On("HostArch").Return("amd64", nil)
	mdInterfaces.On("HostInterfaces").Return(testInterfaces, nil)

	// Create a configuration that enables the HostInterface attribute
	cfg := metadata.DefaultResourceAttributesConfig()
	cfg.HostInterface.Enabled = true

	detector := newTestDetector(mdInterfaces, []string{"os"}, cfg)
	res, schemaURL, err := detector.Detect(context.Background())
	require.NoError(t, err)
	assert.Equal(t, conventions.SchemaURL, schemaURL)
	mdInterfaces.AssertExpectations(t)

	fmt.Println("res.Attributes().AsRaw()", res.Attributes().AsRaw())
	expected := map[string]any{
		string(conventions.HostNameKey): "hostname",
		string(conventions.OSTypeKey):   "linux",
		"host.interface":                testInterfacesAttribute,
	}

	assert.Equal(t, expected, res.Attributes().AsRaw())
}

func TestHostInterfacesError(t *testing.T) {
	mdInterfacesError := &mockMetadata{}
	mdInterfacesError.On("Hostname").Return("hostname", nil)
	mdInterfacesError.On("OSDescription").Return("Ubuntu 22.04.2 LTS (Jammy Jellyfish)", nil)
	mdInterfacesError.On("OSType").Return("linux", nil)
	mdInterfacesError.On("OSVersion").Return("22.04.2 LTS (Jammy Jellyfish)", nil)
	mdInterfacesError.On("HostArch").Return("amd64", nil)
	mdInterfacesError.On("HostInterfaces").Return([]net.Interface{}, errors.New("interface error"))

	// Create a configuration that enables the HostInterface attribute
	cfg := metadata.DefaultResourceAttributesConfig()
	cfg.HostInterface.Enabled = true

	detector := newTestDetector(mdInterfacesError, []string{"os"}, cfg)
	res, schemaURL, err := detector.Detect(context.Background())
	assert.Error(t, err)
	assert.Empty(t, schemaURL)
	assert.True(t, internal.IsEmptyResource(res))
}

func newTestDetector(mock *mockMetadata, hostnameSources []string, resCfg metadata.ResourceAttributesConfig) *Detector {
	return &Detector{
		provider: mock,
		logger:   zap.NewNop(),
		cfg:      Config{HostnameSources: hostnameSources, ResourceAttributes: resCfg},
		rb:       metadata.NewResourceBuilder(resCfg),
	}
}
