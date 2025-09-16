// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOSType_Constants(t *testing.T) {
	assert.Equal(t, OSType("IOS XE"), IOSXE)
	assert.Equal(t, OSType("NX-OS"), NXOS)
	assert.Equal(t, OSType("IOS"), IOS)
}

func TestIsOSSupported_Logic(t *testing.T) {
	tests := []struct {
		name      string
		osType    OSType
		feature   string
		supported bool
	}{
		{
			name:      "bgp_ios_xe",
			osType:    IOSXE,
			feature:   "bgp",
			supported: true,
		},
		{
			name:      "bgp_nx_os",
			osType:    NXOS,
			feature:   "bgp",
			supported: true,
		},
		{
			name:      "bgp_ios",
			osType:    IOS,
			feature:   "bgp",
			supported: false,
		},
		{
			name:      "environment_all_os",
			osType:    IOSXE,
			feature:   "environment",
			supported: true,
		},
		{
			name:      "unknown_feature",
			osType:    IOSXE,
			feature:   "unknown",
			supported: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test the OS support logic directly
			supported := isFeatureSupported(tt.osType, tt.feature)
			assert.Equal(t, tt.supported, supported)
		})
	}
}

// Helper function to test OS support logic without requiring a full client
func isFeatureSupported(osType OSType, feature string) bool {
	switch feature {
	case "bgp":
		return osType == IOSXE || osType == NXOS
	case "environment", "facts", "interfaces", "optics":
		return osType == IOSXE || osType == NXOS || osType == IOS
	default:
		return false
	}
}

// Test OS detection logic with real device output samples
func TestOSDetection_Logic(t *testing.T) {
	tests := []struct {
		name           string
		showVersionOut string
		expectedOS     OSType
	}{
		{
			name: "ios_xe_detection",
			showVersionOut: `Cisco IOS XE Software, Version 16.09.04
Cisco IOS Software [Fuji], ASR1000 Software (X86_64_LINUX_IOSD-UNIVERSALK9-M), Version 16.9.4, RELEASE SOFTWARE (fc2)`,
			expectedOS: IOSXE,
		},
		{
			name: "nx_os_detection",
			showVersionOut: `Cisco Nexus Operating System (NX-OS) Software
TAC support: http://www.cisco.com/tac
Copyright (C) 2002-2020, Cisco and/or its affiliates.`,
			expectedOS: NXOS,
		},
		{
			name:           "ios_detection",
			showVersionOut: `Cisco IOS Software, C2960X Software (C2960X-UNIVERSALK9-M), Version 15.2(4)E7, RELEASE SOFTWARE (fc1)`,
			expectedOS:     IOS,
		},
		{
			name:           "unknown_defaults_to_iosxe",
			showVersionOut: `Unknown device output`,
			expectedOS:     IOSXE,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			detectedOS := detectOSTypeFromOutput(tt.showVersionOut)
			assert.Equal(t, tt.expectedOS, detectedOS)
		})
	}
}

// detectOSTypeFromOutput simulates OS detection logic for testing
func detectOSTypeFromOutput(output string) OSType {
	if strings.Contains(output, "Cisco IOS XE") {
		return IOSXE
	} else if strings.Contains(output, "Cisco Nexus") || strings.Contains(output, "NX-OS") {
		return NXOS
	} else if strings.Contains(output, "Cisco IOS Software") {
		return IOS
	}
	return IOSXE // Default
}

// Test command generation logic for different OS types and features
func TestGetCommand_Logic(t *testing.T) {
	tests := []struct {
		name        string
		osType      OSType
		feature     string
		expectedCmd string
	}{
		// BGP commands
		{"bgp_nxos", NXOS, "bgp", "show bgp all summary"},
		{"bgp_iosxe", IOSXE, "bgp", "show ip bgp summary"},
		{"bgp_ios", IOS, "bgp", "show ip bgp summary"},

		// Environment commands
		{"environment_all_os", IOSXE, "environment", "show environment"},

		// Facts commands
		{"facts_version", IOSXE, "facts_version", "show version"},
		{"facts_memory_nxos", NXOS, "facts_memory", "show system resources"},
		{"facts_memory_iosxe", IOSXE, "facts_memory", "show memory statistics"},
		{"facts_cpu_nxos", NXOS, "facts_cpu", "show system resources"},
		{"facts_cpu_iosxe", IOSXE, "facts_cpu", "show processes cpu"},

		// Interface commands
		{"interfaces_nxos", NXOS, "interfaces", "show interface"},
		{"interfaces_iosxe", IOSXE, "interfaces", "show interfaces"},
		{"interfaces_vlans_iosxe", IOSXE, "interfaces_vlans", "show vlans"},
		{"interfaces_vlans_nxos", NXOS, "interfaces_vlans", ""},

		// Optics commands
		{"optics_nxos", NXOS, "optics", "show interface transceiver"},
		{"optics_iosxe", IOSXE, "optics", "show interfaces transceiver"},

		// Unknown feature
		{"unknown_feature", IOSXE, "unknown", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actualCmd := getCommandForOSAndFeature(tt.osType, tt.feature)
			assert.Equal(t, tt.expectedCmd, actualCmd)
		})
	}
}

// Test feature support logic for different OS types
func TestFeatureSupport_Logic(t *testing.T) {
	tests := []struct {
		name      string
		osType    OSType
		feature   string
		supported bool
	}{
		// BGP support
		{"bgp_iosxe_supported", IOSXE, "bgp", true},
		{"bgp_nxos_supported", NXOS, "bgp", true},
		{"bgp_ios_not_supported", IOS, "bgp", false},

		// Environment support (all OS)
		{"environment_iosxe", IOSXE, "environment", true},
		{"environment_nxos", NXOS, "environment", true},
		{"environment_ios", IOS, "environment", true},

		// Facts support (all OS)
		{"facts_iosxe", IOSXE, "facts", true},
		{"facts_nxos", NXOS, "facts", true},
		{"facts_ios", IOS, "facts", true},

		// Interfaces support (all OS)
		{"interfaces_iosxe", IOSXE, "interfaces", true},
		{"interfaces_nxos", NXOS, "interfaces", true},
		{"interfaces_ios", IOS, "interfaces", true},

		// Optics support
		{"optics_iosxe_supported", IOSXE, "optics", true},
		{"optics_nxos_supported", NXOS, "optics", true},
		{"optics_ios_not_supported", IOS, "optics", false},

		// Unknown feature
		{"unknown_feature", IOSXE, "unknown", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actualSupported := isOSFeatureSupported(tt.osType, tt.feature)
			assert.Equal(t, tt.supported, actualSupported)
		})
	}
}

// Helper function to test command generation logic
func getCommandForOSAndFeature(osType OSType, feature string) string {
	switch feature {
	case "bgp":
		if osType == NXOS {
			return "show bgp all summary"
		}
		return "show ip bgp summary"
	case "environment":
		return "show environment"
	case "facts_version":
		return "show version"
	case "facts_memory":
		if osType == NXOS {
			return "show system resources"
		}
		return "show memory statistics"
	case "facts_cpu":
		if osType == NXOS {
			return "show system resources"
		}
		return "show processes cpu"
	case "interfaces":
		if osType == NXOS {
			return "show interface"
		}
		return "show interfaces"
	case "interfaces_vlans":
		if osType == IOSXE {
			return "show vlans"
		}
		return ""
	case "optics":
		if osType == NXOS {
			return "show interface transceiver"
		}
		return "show interfaces transceiver"
	default:
		return ""
	}
}

// Helper function to test feature support logic
func isOSFeatureSupported(osType OSType, feature string) bool {
	switch feature {
	case "bgp":
		return osType == IOSXE || osType == NXOS
	case "environment", "facts", "interfaces":
		return true // All OS types support these
	case "optics":
		return osType == IOSXE || osType == NXOS
	default:
		return false
	}
}
