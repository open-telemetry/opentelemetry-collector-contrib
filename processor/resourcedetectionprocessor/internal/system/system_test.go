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
			detector, err := NewDetector(processortest.NewNopSettings(processortest.NopType), tt.cfg, false)
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
	res, schemaURL, err := detector.Detect(t.Context())
	require.NoError(t, err)
	assert.Contains(t, schemaURL, "https://opentelemetry.io/schemas/")
	md.AssertExpectations(t)
	md.AssertNotCalled(t, "CPUInfo")

	expected := map[string]any{
		"host.name":      "fqdn",
		"os.description": "Ubuntu 22.04.2 LTS (Jammy Jellyfish)",
		"os.type":        "darwin",
		"os.version":     "22.04.2 LTS (Jammy Jellyfish)",
		"host.id":        "2",
		"host.arch":      "amd64",
		"host.ip":        testIPsAttribute,
		"host.mac":       testMACsAttribute,
		"host.interface": testInterfacesAttribute,
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
	res, schemaURL, err := detector.Detect(t.Context())
	require.NoError(t, err)
	assert.Contains(t, schemaURL, "https://opentelemetry.io/schemas/")
	mdHostname.AssertExpectations(t)
	mdHostname.AssertNotCalled(t, "HostID")
	mdHostname.AssertNotCalled(t, "HostIPs")

	expected := map[string]any{
		"host.name": "hostname",
		"os.type":   "darwin",
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
	res, schemaURL, err := detector.Detect(t.Context())
	require.NoError(t, err)
	assert.Contains(t, schemaURL, "https://opentelemetry.io/schemas/")
	mdHostname.AssertExpectations(t)

	expected := map[string]any{
		"host.name":      "hostname",
		"os.description": "Ubuntu 22.04.2 LTS (Jammy Jellyfish)",
		"os.type":        "darwin",
		"os.version":     "22.04.2 LTS (Jammy Jellyfish)",
		"host.id":        "3",
		"host.arch":      "amd64",
		"host.ip":        testIPsAttribute,
		"host.mac":       testMACsAttribute,
		"host.interface": testInterfacesAttribute,
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
	res, schemaURL, err := detector.Detect(t.Context())
	require.NoError(t, err)
	assert.Contains(t, schemaURL, "https://opentelemetry.io/schemas/")
	mdHostname.AssertExpectations(t)

	expected := map[string]any{
		"host.name":      "hostname",
		"os.description": "Ubuntu 22.04.2 LTS (Jammy Jellyfish)",
		"os.type":        "darwin",
		"os.version":     "22.04.2 LTS (Jammy Jellyfish)",
		"host.id":        "1",
		"host.arch":      "amd64",
		"host.ip":        testIPsAttribute,
		"host.mac":       testMACsAttribute,
		"host.interface": testInterfacesAttribute,
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
	res, schemaURL, err := detector.Detect(t.Context())
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
	res, schemaURL, err = detector.Detect(t.Context())
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
	res, schemaURL, err = detector.Detect(t.Context())
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
	res, schemaURL, err = detector.Detect(t.Context())
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
	res, schemaURL, err = detector.Detect(t.Context())
	assert.NoError(t, err)
	assert.Contains(t, schemaURL, "https://opentelemetry.io/schemas/")
	assert.Equal(t, map[string]any{
		"host.name":      "hostname",
		"os.description": "Ubuntu 22.04.2 LTS (Jammy Jellyfish)",
		"os.type":        "linux",
		"os.version":     "22.04.2 LTS (Jammy Jellyfish)",
		"host.arch":      "arm64",
		"host.ip":        testIPsAttribute,
		"host.mac":       testMACsAttribute,
		"host.interface": testInterfacesAttribute,
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
	res, schemaURL, err := detector.Detect(t.Context())
	require.NoError(t, err)
	assert.Contains(t, schemaURL, "https://opentelemetry.io/schemas/")
	md.AssertExpectations(t)

	expected := map[string]any{
		"host.name":       "fqdn",
		"os.description":  "Ubuntu 22.04.2 LTS (Jammy Jellyfish)",
		"os.type":         "darwin",
		"os.version":      "22.04.2 LTS (Jammy Jellyfish)",
		"host.id":         "2",
		"host.arch":       "amd64",
		"host.ip":         testIPsAttribute,
		"host.mac":        testMACsAttribute,
		"host.cpu.family": "some",
		"host.interface":  testInterfacesAttribute,
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
	res, _, err := detector.Detect(t.Context())
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
	res, schemaURL, err := detector.Detect(t.Context())
	require.NoError(t, err)
	assert.Contains(t, schemaURL, "https://opentelemetry.io/schemas/")
	mdInterfaces.AssertExpectations(t)

	fmt.Println("res.Attributes().AsRaw()", res.Attributes().AsRaw())
	expected := map[string]any{
		"host.name":      "hostname",
		"os.type":        "linux",
		"host.interface": testInterfacesAttribute,
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
	res, schemaURL, err := detector.Detect(t.Context())
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

// newFullMock returns a mock where every provider call succeeds.
func newFullMock() *mockMetadata {
	md := &mockMetadata{}
	md.On("Hostname").Return("hostname", nil).Maybe()
	md.On("FQDN").Return("fqdn", nil).Maybe()
	md.On("LookupCNAME").Return("cname", nil).Maybe()
	md.On("ReverseLookupHost").Return("reverse", nil).Maybe()
	md.On("OSDescription").Return("Ubuntu 22.04.2 LTS (Jammy Jellyfish)", nil).Maybe()
	md.On("OSType").Return("linux", nil).Maybe()
	md.On("OSVersion").Return("22.04.2", nil).Maybe()
	md.On("OSName").Return("Ubuntu", nil).Maybe()
	md.On("OSBuildID").Return("22H2", nil).Maybe()
	md.On("HostID").Return("1", nil).Maybe()
	md.On("HostArch").Return("amd64", nil).Maybe()
	md.On("HostIPs").Return(testIPsAddresses, nil).Maybe()
	md.On("HostMACs").Return(testMACsAddresses, nil).Maybe()
	md.On("HostInterfaces").Return(testInterfaces, nil).Maybe()
	md.On("CPUInfo").Return([]cpu.InfoStat{{VendorID: "GenuineIntel", Family: "6", ModelName: "Intel", Stepping: 1, CacheSize: 256}}, nil).Maybe()
	return md
}

// newFailingMock returns a mock where errMethod fails with err and every other call succeeds.
// testify matches expectations in order, so the failing one is registered first.
func newFailingMock(errMethod string, err error) *mockMetadata {
	md := &mockMetadata{}
	switch errMethod {
	case "HostIPs":
		md.On(errMethod).Return([]net.IP(nil), err)
	case "HostMACs":
		md.On(errMethod).Return([]net.HardwareAddr(nil), err)
	case "HostInterfaces":
		md.On(errMethod).Return([]net.Interface(nil), err)
	case "CPUInfo":
		md.On(errMethod).Return([]cpu.InfoStat(nil), err)
	default:
		md.On(errMethod).Return("", err)
	}
	full := newFullMock()
	md.ExpectedCalls = append(md.ExpectedCalls, full.ExpectedCalls...)
	return md
}

func TestDetectProviderErrors(t *testing.T) {
	someErr := errors.New("boom")
	tests := []struct {
		method  string
		wantErr string
	}{
		{method: "OSType", wantErr: "failed getting OS type"},
		{method: "OSVersion", wantErr: "failed getting OS version"},
		{method: "HostArch", wantErr: "failed getting host architecture"},
		{method: "HostIPs", wantErr: "failed getting host IP addresses"},
		{method: "HostMACs", wantErr: "failed to get host MAC addresses"},
		{method: "HostInterfaces", wantErr: "failed to get host network interfaces"},
		{method: "OSDescription", wantErr: "failed getting OS description"},
		{method: "CPUInfo", wantErr: "failed getting host cpuinfo"},
	}
	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			cfg := allEnabledConfig()
			cfg.HostCPUVendorID.Enabled = true

			detector := newTestDetector(newFailingMock(tt.method, someErr), []string{"os"}, cfg)
			res, schemaURL, err := detector.Detect(t.Context())
			require.ErrorIs(t, err, someErr)
			assert.ErrorContains(t, err, tt.wantErr)
			assert.Empty(t, schemaURL)
			assert.True(t, internal.IsEmptyResource(res))
		})
	}
}

func TestDetectHostnameSources(t *testing.T) {
	someErr := errors.New("boom")
	tests := []struct {
		name     string
		sources  []string
		md       *mockMetadata
		expected string
	}{
		{name: "cname", sources: []string{"cname"}, md: newFullMock(), expected: "cname"},
		{name: "lookup", sources: []string{"lookup"}, md: newFullMock(), expected: "reverse"},
		{name: "cname fails, falls back to lookup", sources: []string{"cname", "lookup"}, md: newFailingMock("LookupCNAME", someErr), expected: "reverse"},
		{name: "lookup fails, falls back to os", sources: []string{"lookup", "os"}, md: newFailingMock("ReverseLookupHost", someErr), expected: "hostname"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resCfg := metadata.DefaultResourceAttributesConfig()
			detector := newTestDetector(tt.md, tt.sources, resCfg)
			res, _, err := detector.Detect(t.Context())
			require.NoError(t, err)
			hostName, ok := res.Attributes().Get("host.name")
			require.True(t, ok)
			assert.Equal(t, tt.expected, hostName.Str())
		})
	}
}

func TestDetectOptionalAttributeErrors(t *testing.T) {
	someErr := errors.New("boom")
	for _, method := range []string{"OSName", "OSBuildID"} {
		t.Run(method, func(t *testing.T) {
			resCfg := metadata.DefaultResourceAttributesConfig()
			resCfg.OsName.Enabled = true
			resCfg.OsBuildID.Enabled = true

			detector := newTestDetector(newFailingMock(method, someErr), []string{"os"}, resCfg)
			res, _, err := detector.Detect(t.Context())
			require.NoError(t, err)
			_, hasOSName := res.Attributes().Get("os.name")
			_, hasOSBuildID := res.Attributes().Get("os.build.id")
			assert.Equal(t, method != "OSName", hasOSName)
			assert.Equal(t, method != "OSBuildID", hasOSBuildID)
		})
	}
}

func TestDetectCPUInfoModelID(t *testing.T) {
	tests := []struct {
		name     string
		model    string
		expected map[string]any
	}{
		{name: "model set", model: "85", expected: map[string]any{"host.cpu.model.id": "85", "host.cpu.vendor.id": "GenuineIntel"}},
		// Windows leaves the model blank, see https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/27675
		{name: "model empty", model: "", expected: map[string]any{"host.cpu.vendor.id": "GenuineIntel"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resCfg := metadata.ResourceAttributesConfig{}
			resCfg.HostCPUModelID.Enabled = true
			resCfg.HostCPUVendorID.Enabled = true

			md := &mockMetadata{}
			md.On("CPUInfo").Return([]cpu.InfoStat{{VendorID: "GenuineIntel", Model: tt.model}}, nil)
			md.ExpectedCalls = append(md.ExpectedCalls, newFullMock().ExpectedCalls...)

			detector := newTestDetector(md, []string{"os"}, resCfg)
			res, _, err := detector.Detect(t.Context())
			require.NoError(t, err)
			assert.Equal(t, tt.expected, res.Attributes().AsRaw())
		})
	}
}

func TestNewDetectorDefaultHostnameSources(t *testing.T) {
	d, err := NewDetector(processortest.NewNopSettings(processortest.NopType), CreateDefaultConfig(), false)
	require.NoError(t, err)
	assert.Equal(t, []string{"dns", "os"}, d.(*Detector).cfg.HostnameSources)
}

func TestValidate(t *testing.T) {
	valid := Config{HostnameSources: []string{"os", "dns", "cname", "lookup"}}
	require.NoError(t, valid.Validate())

	invalid := Config{HostnameSources: []string{"os", "bogus"}}
	require.EqualError(t, invalid.Validate(), `hostname_sources contains invalid value: "bogus"`)
}

func TestCreateDefaultConfig(t *testing.T) {
	assert.Equal(t, Config{ResourceAttributes: metadata.DefaultResourceAttributesConfig()}, CreateDefaultConfig())
}
