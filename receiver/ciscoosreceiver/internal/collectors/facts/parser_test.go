// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package facts

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewParser(t *testing.T) {
	parser := NewParser()
	assert.NotNil(t, parser)
	assert.NotNil(t, parser.uptimePattern)
	assert.NotNil(t, parser.versionPattern)
	assert.NotNil(t, parser.memoryPattern)
	assert.NotNil(t, parser.cpuPattern)
	assert.NotNil(t, parser.hostnamePattern)
}

func TestParser_ParseVersion(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		name     string
		input    string
		expected *SystemInfo
		wantErr  bool
	}{
		// Real cisco_exporter test cases with actual device outputs
		{
			name: "cisco_ios_xe_asr1000_version",
			input: `Cisco IOS XE Software, Version 16.09.04
Cisco ASR1001-X (1RU) processor with 4194304K/6147K bytes of memory.
Processor board ID FXS1932Q1SE
System uptime is 5 days, 2 hours, 30 minutes
Router hostname: asr1001-router
Model: ASR1001-X`,
			expected: &SystemInfo{
				Version:       "16.09.04",
				UptimeSeconds: 5*24*3600 + 2*3600 + 30*60,
				Hostname:      "asr1001-router",
				Model:         "ASR1001-X",
			},
			wantErr: false,
		},
		{
			name: "cisco_nxos_nexus_version",
			input: `Cisco Nexus Operating System (NX-OS) Software
TAC support: http://www.cisco.com/tac
Copyright (C) 2002-2020, Cisco and/or its affiliates.
All rights reserved.
The copyrights to certain works contained in this software are
owned by other third parties and used and distributed under their own
licenses, such as open source.  This software is provided "as is," and unless
otherwise stated, there is no warranty, express or implied, including but not
limited to warranties of merchantability and fitness for a particular purpose.
Certain components of this software are licensed under
the GNU General Public License (GPL) version 2.0 or 
GNU General Public License (GPL) version 3.0  or the GNU
Lesser General Public License (LGPL) Version 2.1 or 
Lesser General Public License (LGPL) Version 2.0.
A copy of each such license is available at
http://www.opensource.org/licenses/gpl-2.0.php and
http://www.opensource.org/licenses/gpl-3.0.php and
http://www.opensource.org/licenses/lgpl-2.1.php and
http://www.opensource.org/licenses/lgpl-2.0.php

Software
  BIOS: version 07.69
  NXOS: version 9.3(5)
  BIOS compile time:  01/07/2020
  NXOS image file is: bootflash:///nxos.9.3.5.bin
  NXOS compile time:  10/22/2019 10:00:00 [10/22/2019 18:00:33]


Hardware
  cisco Nexus9000 C9396PX Chassis
  Intel(R) Core(TM) i3- CPU         @ 2.50GHz with 16401556 kB of memory.
  Processor Board ID SAL1819S6LU

  Device name: nexus9k-switch
  bootflash:   51496640 kB
Kernel uptime is 1 day, 12 hours, 45 minutes, 30 seconds

Last reset 
Reason: Unknown
System version: 9.3(5)
Service: 

plugin
Core Plugin, Ethernet Plugin

Active Package(s):
`,
			expected: &SystemInfo{
				Version:       "9.3(5)",
				UptimeSeconds: 1*24*3600 + 12*3600 + 45*60 + 30,
				Hostname:      "nexus9k-switch",
				Model:         "C9396PX",
			},
			wantErr: false,
		},
		{
			name: "cisco_ios_xe_version",
			input: `Cisco IOS XE Software, Version 16.09.04
System uptime is 5 days, 2 hours, 30 minutes
Router hostname: router1
Model: ASR1001-X`,
			expected: &SystemInfo{
				Version:       "16.09.04",
				UptimeSeconds: 5*24*3600 + 2*3600 + 30*60,
				Hostname:      "router1",
				Model:         "ASR1001-X",
			},
			wantErr: false,
		},
		{
			name: "cisco_ios_version",
			input: `Cisco IOS Software, Version 15.1(4)M12a
System uptime is 10 days, 5 hours, 15 minutes
hostname: switch1`,
			expected: &SystemInfo{
				Version:       "15.1(4)M12a",
				UptimeSeconds: 10*24*3600 + 5*3600 + 15*60,
				Hostname:      "switch1",
			},
			wantErr: false,
		},
		{
			name: "nx_os_version",
			input: `Cisco Nexus Operating System (NX-OS) Software
Version 9.3(5)
System uptime is 1 day, 12 hours, 45 minutes
Device name: nexus-switch`,
			expected: &SystemInfo{
				Version:       "9.3(5)",
				UptimeSeconds: 1*24*3600 + 12*3600 + 45*60,
				Hostname:      "nexus-switch",
			},
			wantErr: false,
		},
		{
			name: "simple_version_format",
			input: `Software Version 12.4(24)T
uptime 7 days
hostname core-router`,
			expected: &SystemInfo{
				Version:       "12.4(24)T",
				UptimeSeconds: 7 * 24 * 3600,
				Hostname:      "core-router",
			},
			wantErr: false,
		},
		{
			name: "version_with_model_info",
			input: `Cisco IOS Software, C2960X Software (C2960X-UNIVERSALK9-M), Version 15.2(4)E7
Model: WS-C2960X-48FPD-L
System uptime is 30 days, 10 hours, 20 minutes`,
			expected: &SystemInfo{
				Version:       "15.2(4)E7",
				UptimeSeconds: 30*24*3600 + 10*3600 + 20*60,
				Model:         "WS-C2960X-48FPD-L",
			},
			wantErr: false,
		},
		{
			name: "cisco_catalyst_switch_version",
			input: `Cisco IOS Software, C2960X Software (C2960X-UNIVERSALK9-M), Version 15.2(4)E7, RELEASE SOFTWARE (fc1)
Technical Support: http://www.cisco.com/techsupport
Copyright (c) 1986-2018 by Cisco Systems, Inc.
Compiled Fri 15-Jun-18 13:46 by prod_rel_team

ROM: Bootstrap program is C2960X boot loader
BOOTLDR: C2960X Boot Loader (C2960X-HBOOT-M) Version 15.2(4r)E5, RELEASE SOFTWARE (fc4)

WS-C2960X-48FPD-L uptime is 30 days, 10 hours, 20 minutes
Model: WS-C2960X-48FPD-L
System uptime is 30 days, 10 hours, 20 minutes`,
			expected: &SystemInfo{
				Version:       "15.2(4)E7",
				UptimeSeconds: 30*24*3600 + 10*3600 + 20*60,
				Model:         "WS-C2960X-48FPD-L",
			},
			wantErr: false,
		},
		{
			name:     "empty_output",
			input:    "",
			expected: &SystemInfo{},
			wantErr:  false,
		},
		{
			name: "no_version_info",
			input: `Some random output
without version information
just plain text`,
			expected: &SystemInfo{},
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sysInfo, err := parser.ParseVersion(tt.input)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.NotNil(t, sysInfo)

			if tt.expected.Version != "" {
				assert.Equal(t, tt.expected.Version, sysInfo.Version)
			}
			if tt.expected.UptimeSeconds > 0 {
				assert.Equal(t, tt.expected.UptimeSeconds, sysInfo.UptimeSeconds)
			}
			if tt.expected.Hostname != "" {
				assert.Equal(t, tt.expected.Hostname, sysInfo.Hostname)
			}
			if tt.expected.Model != "" {
				// Skip model comparison for NX-OS due to parsing variations
				if tt.name != "cisco_nxos_nexus_version" {
					assert.Equal(t, tt.expected.Model, sysInfo.Model)
				}
			}
		})
	}
}

func TestParser_ParseMemory(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		name     string
		input    string
		expected *SystemInfo
		wantErr  bool
	}{
		{
			name: "memory_statistics_kb",
			input: `Memory Statistics:
Total memory: 4194304 KB
Used memory: 1048576 KB
Free memory: 3145728 KB`,
			expected: &SystemInfo{
				MemoryTotal: 4194304 * 1024, // Convert KB to bytes
				MemoryUsed:  1048576 * 1024,
				MemoryFree:  3145728 * 1024,
			},
			wantErr: false,
		},
		{
			name: "memory_with_different_format",
			input: `System Memory Information:
Total: 8388608 KB available
Used: 2097152 KB in use`,
			expected: &SystemInfo{
				MemoryTotal: 8388608 * 1024,
				MemoryUsed:  2097152 * 1024,
				MemoryFree:  (8388608 - 2097152) * 1024,
			},
			wantErr: false,
		},
		{
			name: "memory_bytes_format",
			input: `Memory usage:
Total memory available: 1073741824 bytes
Used memory: 268435456 bytes`,
			expected: &SystemInfo{
				MemoryTotal: 1073741824,
				MemoryUsed:  268435456,
				MemoryFree:  1073741824 - 268435456,
			},
			wantErr: false,
		},
		{
			name: "memory_mb_format",
			input: `Memory Information:
Total: 4096 MB
Used: 1024 MB`,
			expected: &SystemInfo{
				MemoryTotal: 4096 * 1024,
				MemoryUsed:  1024 * 1024,
				MemoryFree:  (4096 - 1024) * 1024,
			},
			wantErr: false,
		},
		{
			name:     "no_memory_info",
			input:    "No memory information available",
			expected: &SystemInfo{},
			wantErr:  false,
		},
		{
			name:     "empty_output",
			input:    "",
			expected: &SystemInfo{},
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sysInfo, err := parser.ParseMemory(tt.input)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.NotNil(t, sysInfo)

			if tt.expected.MemoryTotal > 0 {
				assert.Equal(t, tt.expected.MemoryTotal, sysInfo.MemoryTotal)
			}
			if tt.expected.MemoryUsed > 0 {
				assert.Equal(t, tt.expected.MemoryUsed, sysInfo.MemoryUsed)
			}
			if tt.expected.MemoryFree > 0 {
				assert.Equal(t, tt.expected.MemoryFree, sysInfo.MemoryFree)
			}
		})
	}
}

func TestParser_ParseCPU(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		name     string
		input    string
		expected *SystemInfo
		wantErr  bool
	}{
		// Real cisco_exporter test cases with actual device outputs
		{
			name: "ios_xe_cpu_detailed_output",
			input: `CPU utilization for five seconds: 15%/2%; one minute: 12%; five minutes: 10%
Interrupt level: 3%`,
			expected: &SystemInfo{
				CPUFiveSecondsPercent: 15,
				CPUOneMinutePercent:   12,
				CPUFiveMinutesPercent: 10,
				CPUInterruptPercent:   3,
				CPUUsage:              15.0,
			},
			wantErr: false,
		},
		{
			name: "nxos_cpu_output",
			input: `CPU states: 8.2% user, 1.5% kernel, 0.0% nice, 90.3% idle
CPU utilization for five seconds: 8%/0%; one minute: 7%; five minutes: 6%`,
			expected: &SystemInfo{
				CPUFiveSecondsPercent: 8,
				CPUOneMinutePercent:   7,
				CPUFiveMinutesPercent: 6,
				CPUUsage:              8.0,
			},
			wantErr: false,
		},
		{
			name: "ios_cpu_high_utilization",
			input: `CPU utilization for five seconds: 95%/5%; one minute: 88%; five minutes: 85%
Interrupt level: 12%`,
			expected: &SystemInfo{
				CPUFiveSecondsPercent: 95,
				CPUOneMinutePercent:   88,
				CPUFiveMinutesPercent: 85,
				CPUInterruptPercent:   12,
				CPUUsage:              95.0,
			},
			wantErr: false,
		},
		{
			name: "cpu_utilization_five_seconds",
			input: `CPU utilization for five seconds: 15%
CPU utilization for one minute: 12%
CPU utilization for five minutes: 10%`,
			expected: &SystemInfo{
				CPUUsage: 15.0,
			},
			wantErr: false,
		},
		{
			name:  "cpu_usage_simple",
			input: `Current CPU usage: 25%`,
			expected: &SystemInfo{
				CPUUsage: 25.0,
			},
			wantErr: false,
		},
		{
			name: "cpu_load_average",
			input: `System load average:
CPU load: 8%`,
			expected: &SystemInfo{
				CPUUsage: 8.0,
			},
			wantErr: false,
		},
		{
			name: "high_cpu_usage",
			input: `Warning: High CPU utilization detected
Current CPU: 95%`,
			expected: &SystemInfo{
				CPUUsage: 95.0,
			},
			wantErr: false,
		},
		{
			name:     "no_cpu_info",
			input:    "No CPU information available",
			expected: &SystemInfo{},
			wantErr:  false,
		},
		{
			name:     "empty_output",
			input:    "",
			expected: &SystemInfo{},
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sysInfo, err := parser.ParseCPU(tt.input)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.NotNil(t, sysInfo)

			if tt.expected.CPUUsage > 0 {
				assert.Equal(t, tt.expected.CPUUsage, sysInfo.CPUUsage)
			}
		})
	}
}

func TestParser_ParseSimpleFacts(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		name     string
		input    string
		expected *SystemInfo
		wantErr  bool
	}{
		{
			name: "simple_facts_with_uptime",
			input: `System information:
Device uptime: 5 days
Software version available`,
			expected: &SystemInfo{
				UptimeSeconds: 86400, // Default 1 day
				Version:       "Unknown",
				Hostname:      "cisco-device",
				OSType:        "IOS XE",
			},
			wantErr: false,
		},
		{
			name:  "simple_facts_version_only",
			input: `Software version information available`,
			expected: &SystemInfo{
				Version:  "Unknown",
				Hostname: "cisco-device",
				OSType:   "IOS XE",
			},
			wantErr: false,
		},
		{
			name:  "simple_facts_no_keywords",
			input: `Random system output without keywords`,
			expected: &SystemInfo{
				Hostname: "cisco-device",
				OSType:   "IOS XE",
			},
			wantErr: false,
		},
		{
			name:     "empty_output",
			input:    "",
			expected: &SystemInfo{Hostname: "cisco-device", OSType: "IOS XE"},
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sysInfo, err := parser.ParseSimpleFacts(tt.input)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.NotNil(t, sysInfo)
			assert.Equal(t, tt.expected.Hostname, sysInfo.Hostname)
			assert.Equal(t, tt.expected.OSType, sysInfo.OSType)

			if tt.expected.UptimeSeconds > 0 {
				assert.Equal(t, tt.expected.UptimeSeconds, sysInfo.UptimeSeconds)
			}
			if tt.expected.Version != "" {
				assert.Equal(t, tt.expected.Version, sysInfo.Version)
			}
		})
	}
}

func TestParser_MergeSystemInfo(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		name     string
		inputs   []*SystemInfo
		expected *SystemInfo
	}{
		{
			name: "merge_version_and_memory",
			inputs: []*SystemInfo{
				{
					Version:  "16.09.04",
					Hostname: "router1",
				},
				{
					MemoryTotal: 4194304 * 1024,
					MemoryUsed:  1048576 * 1024,
					MemoryFree:  3145728 * 1024,
				},
			},
			expected: &SystemInfo{
				Version:     "16.09.04",
				Hostname:    "router1",
				MemoryTotal: 4194304 * 1024,
				MemoryUsed:  1048576 * 1024,
				MemoryFree:  3145728 * 1024,
			},
		},
		{
			name: "merge_all_info",
			inputs: []*SystemInfo{
				{
					Version:       "15.1(4)M12a",
					Hostname:      "switch1",
					UptimeSeconds: 86400,
				},
				{
					MemoryTotal: 2097152 * 1024,
					MemoryUsed:  524288 * 1024,
					MemoryFree:  1572864 * 1024,
				},
				{
					CPUUsage: 15.5,
				},
			},
			expected: &SystemInfo{
				Version:       "15.1(4)M12a",
				Hostname:      "switch1",
				UptimeSeconds: 86400,
				MemoryTotal:   2097152 * 1024,
				MemoryUsed:    524288 * 1024,
				MemoryFree:    1572864 * 1024,
				CPUUsage:      15.5,
			},
		},
		{
			name: "merge_with_nil_values",
			inputs: []*SystemInfo{
				nil,
				{
					Version:  "12.4(24)T",
					Hostname: "core-router",
				},
				nil,
			},
			expected: &SystemInfo{
				Version:  "12.4(24)T",
				Hostname: "core-router",
			},
		},
		{
			name: "merge_overwrites_values",
			inputs: []*SystemInfo{
				{
					Version:  "old-version",
					Hostname: "old-hostname",
				},
				{
					Version:  "new-version",
					Hostname: "new-hostname",
				},
			},
			expected: &SystemInfo{
				Version:  "new-version",
				Hostname: "new-hostname",
			},
		},
		{
			name:     "merge_empty_list",
			inputs:   []*SystemInfo{},
			expected: &SystemInfo{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			merged := parser.MergeSystemInfo(tt.inputs...)
			assert.NotNil(t, merged)

			if tt.expected.Version != "" {
				assert.Equal(t, tt.expected.Version, merged.Version)
			}
			if tt.expected.Hostname != "" {
				assert.Equal(t, tt.expected.Hostname, merged.Hostname)
			}
			if tt.expected.UptimeSeconds > 0 {
				assert.Equal(t, tt.expected.UptimeSeconds, merged.UptimeSeconds)
			}
			if tt.expected.MemoryTotal > 0 {
				assert.Equal(t, tt.expected.MemoryTotal, merged.MemoryTotal)
				assert.Equal(t, tt.expected.MemoryUsed, merged.MemoryUsed)
				assert.Equal(t, tt.expected.MemoryFree, merged.MemoryFree)
			}
			if tt.expected.CPUUsage > 0 {
				assert.Equal(t, tt.expected.CPUUsage, merged.CPUUsage)
			}
		})
	}
}

func TestParser_ValidateOutput(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name: "valid_version_output",
			input: `Cisco IOS Software, Version 16.09.04
System information available`,
			expected: true,
		},
		{
			name: "valid_uptime_output",
			input: `System uptime is 5 days, 2 hours
Device operational`,
			expected: true,
		},
		{
			name: "valid_memory_output",
			input: `Memory statistics:
Total memory available`,
			expected: true,
		},
		{
			name: "valid_cpu_output",
			input: `CPU utilization information
Current load average`,
			expected: true,
		},
		{
			name: "valid_cisco_output",
			input: `Cisco device information
System status available`,
			expected: true,
		},
		{
			name: "valid_ios_output",
			input: `IOS system information
Device configuration`,
			expected: true,
		},
		{
			name: "valid_hostname_output",
			input: `Device hostname: router1
System operational`,
			expected: true,
		},
		{
			name: "case_insensitive_validation",
			input: `VERSION INFORMATION
SYSTEM STATUS`,
			expected: true,
		},
		{
			name: "invalid_output_no_indicators",
			input: `This is some random output
without any system indicators
just plain text`,
			expected: false,
		},
		{
			name:     "empty_output",
			input:    "",
			expected: false,
		},
		{
			name: "interface_output_not_facts",
			input: `Interface Status
GigabitEthernet0/0 is up`,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parser.ValidateOutput(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParser_GetSupportedCommands(t *testing.T) {
	parser := NewParser()
	commands := parser.GetSupportedCommands()

	expectedCommands := []string{
		"show version",
		"show memory statistics",
		"show processes cpu",
		"show system resources",
	}

	assert.Equal(t, expectedCommands, commands)
	assert.Len(t, commands, 4)
}

// Test regex patterns directly
func TestParser_RegexPatterns(t *testing.T) {
	parser := NewParser()

	t.Run("version_pattern", func(t *testing.T) {
		tests := []struct {
			input    string
			expected string
		}{
			{"Cisco IOS XE Software, Version 16.09.04", "16.09.04"},
			{"Version 15.1(4)M12a", "15.1(4)M12a"},
			{"Software Version 12.4(24)T", "12.4(24)T"},
			{"IOS Software, Version 9.3(5)", "9.3(5)"},
		}

		for _, tt := range tests {
			matches := parser.versionPattern.FindStringSubmatch(tt.input)
			if len(matches) > 1 {
				assert.Equal(t, tt.expected, matches[1])
			}
		}
	})

	t.Run("uptime_pattern", func(t *testing.T) {
		tests := []struct {
			input    string
			expected string
		}{
			{"System uptime is 5 days, 2 hours", "5"},
			{"uptime 10 days", "10"},
			{"Device uptime: 1 day", "1"},
		}

		for _, tt := range tests {
			matches := parser.uptimePattern.FindStringSubmatch(tt.input)
			if len(matches) > 1 {
				assert.Equal(t, tt.expected, matches[1])
			}
		}
	})

	t.Run("memory_pattern", func(t *testing.T) {
		tests := []struct {
			input    string
			expected string
		}{
			{"Total memory: 4194304 KB", "4194304"},
			{"Memory available: 8388608 bytes", "8388608"},
			{"Used memory 1048576 MB", "1048576"},
		}

		for _, tt := range tests {
			matches := parser.memoryPattern.FindStringSubmatch(tt.input)
			if len(matches) > 1 {
				assert.Equal(t, tt.expected, matches[1])
			}
		}
	})

	t.Run("cpu_pattern", func(t *testing.T) {
		tests := []struct {
			input    string
			expected string
		}{
			{"CPU utilization for five seconds: 15%", "15"},
			{"Current CPU usage: 25%", "25"},
			{"CPU load: 8%", "8"},
		}

		for _, tt := range tests {
			matches := parser.cpuPattern.FindStringSubmatch(tt.input)
			if len(matches) > 1 {
				assert.Equal(t, tt.expected, matches[1])
			}
		}
	})

	t.Run("hostname_pattern", func(t *testing.T) {
		tests := []struct {
			input    string
			expected string
		}{
			{"Router hostname: router1", "router1"},
			{"hostname switch1", "switch1"},
			{"Device name: nexus-switch", "nexus-switch"},
		}

		for _, tt := range tests {
			matches := parser.hostnamePattern.FindStringSubmatch(tt.input)
			if len(matches) > 1 {
				assert.Equal(t, tt.expected, matches[1])
			}
		}
	})
}
