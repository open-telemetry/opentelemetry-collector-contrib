// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package systemscraper

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseCPUUtilizationNXOS(t *testing.T) {
	tests := []struct {
		name           string
		output         string
		expectedCPU    float64
		expectedErr    bool
		errDescription string
	}{
		{
			name: "Valid NX-OS output - high CPU",
			output: `Load average:   1 minute: 3.02   5 minutes: 3.26   15 minutes: 3.25
Processes   :   903 total, 5 running
CPU states  :   27.70% user,   10.80% kernel,   61.49% idle
        CPU0 states  :   23.00% user,   7.00% kernel,   70.00% idle
        CPU1 states  :   6.31% user,   10.52% kernel,   83.15% idle
Memory usage:   32803148K total,   11174924K used,   21628224K free`,
			expectedCPU: 0.385, // 27.70 + 10.80 = 38.5%
			expectedErr: false,
		},
		{
			name: "Valid NX-OS output - low CPU",
			output: `Load average:   1 minute: 0.12   5 minutes: 0.15   15 minutes: 0.18
Processes   :   850 total, 2 running
CPU states  :   2.00% user,   1.00% kernel,   97.00% idle
Memory usage:   8183648K total,   2000000K used,   6183648K free`,
			expectedCPU: 0.03, // 2.00 + 1.00 = 3.0%
			expectedErr: false,
		},
		{
			name: "Valid NX-OS output - zero CPU",
			output: `Load average:   1 minute: 0.00   5 minutes: 0.00   15 minutes: 0.00
Processes   :   800 total, 1 running
CPU states  :   0.00% user,   0.00% kernel,   100.00% idle
Memory usage:   32803148K total,   8000000K used,   24803148K free`,
			expectedCPU: 0.00, // 0.00 + 0.00 = 0.0%
			expectedErr: false,
		},
		{
			name:           "Invalid output - missing CPU states line",
			output:         `Load average:   1 minute: 1.00   5 minutes: 1.00   15 minutes: 1.00\nProcesses   :   900 total, 3 running`,
			expectedCPU:    0,
			expectedErr:    true,
			errDescription: "should fail when CPU states line is missing",
		},
		{
			name:           "Invalid output - empty string",
			output:         "",
			expectedCPU:    0,
			expectedErr:    true,
			errDescription: "should fail for empty output",
		},
		{
			name: "Invalid output - malformed CPU line",
			output: `Load average:   1 minute: 1.00
CPU states  :   invalid% user,   invalid% kernel,   97.00% idle`,
			expectedCPU:    0,
			expectedErr:    true,
			errDescription: "should fail when CPU percentages are malformed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cpuUtil, err := parseCPUUtilizationNXOS(tt.output)

			if tt.expectedErr {
				require.Error(t, err, tt.errDescription)
			} else {
				require.NoError(t, err)
				assert.InDelta(t, tt.expectedCPU, cpuUtil, 0.001, "CPU utilization should match")
			}
		})
	}
}

func TestParseCPUUtilizationIOS(t *testing.T) {
	tests := []struct {
		name           string
		output         string
		expectedCPU    float64
		expectedErr    bool
		errDescription string
	}{
		{
			name: "Valid IOS XE output",
			output: `CPU utilization for five seconds: 8%/2%; one minute: 7%; five minutes: 6%
PID Runtime(ms)     Invoked      uSecs   5Sec   1Min   5Min TTY Process
  1           0          12          0  0.00%  0.00%  0.00%   0 Chunk Manager`,
			expectedCPU: 0.08, // 8%
			expectedErr: false,
		},
		{
			name: "Valid IOS output - high CPU",
			output: `CPU utilization for five seconds: 45%/12%; one minute: 40%; five minutes: 38%
PID Runtime(ms)     Invoked      uSecs   5Sec   1Min   5Min TTY Process`,
			expectedCPU: 0.45, // 45%
			expectedErr: false,
		},
		{
			name: "Valid IOS output - zero CPU",
			output: `CPU utilization for five seconds: 0%/0%; one minute: 0%; five minutes: 0%
PID Runtime(ms)     Invoked      uSecs   5Sec   1Min   5Min TTY Process`,
			expectedCPU: 0.00, // 0%
			expectedErr: false,
		},
		{
			name:           "Invalid output - missing CPU utilization line",
			output:         `PID Runtime(ms)     Invoked      uSecs   5Sec   1Min   5Min TTY Process`,
			expectedCPU:    0,
			expectedErr:    true,
			errDescription: "should fail when CPU utilization line is missing",
		},
		{
			name:           "Invalid output - empty string",
			output:         "",
			expectedCPU:    0,
			expectedErr:    true,
			errDescription: "should fail for empty output",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cpuUtil, err := parseCPUUtilizationIOS(tt.output)

			if tt.expectedErr {
				require.Error(t, err, tt.errDescription)
			} else {
				require.NoError(t, err)
				assert.InDelta(t, tt.expectedCPU, cpuUtil, 0.001, "CPU utilization should match")
			}
		})
	}
}

func TestParseMemoryUtilizationNXOS(t *testing.T) {
	tests := []struct {
		name           string
		output         string
		expectedMem    float64
		expectedErr    bool
		errDescription string
	}{
		{
			name: "Valid NX-OS output - actual device",
			output: `Load average:   1 minute: 3.02   5 minutes: 3.26   15 minutes: 3.25
Processes   :   903 total, 5 running
CPU states  :   27.70% user,   10.80% kernel,   61.49% idle
Memory usage:   32803148K total,   11174924K used,   21628224K free
Kernel vmalloc:   0K total,   0K free
Kernel buffers:   596304K Used
Kernel cached :   6625608K Used
Current memory status: OK`,
			expectedMem: 0.3406, // 11174924 / 32803148 = 0.3406
			expectedErr: false,
		},
		{
			name: "Valid NX-OS output - low memory usage",
			output: `CPU states  :   2.00% user,   1.00% kernel,   97.00% idle
Memory usage:   32803148K total,   8183648K used,   24619500K free
Current memory status: OK`,
			expectedMem: 0.2494, // 8183648 / 32803148 = 0.2494
			expectedErr: false,
		},
		{
			name: "Valid NX-OS output - high memory usage",
			output: `CPU states  :   10.00% user,   5.00% kernel,   85.00% idle
Memory usage:   32803148K total,   29522835K used,   3280313K free
Current memory status: Warning`,
			expectedMem: 0.9000, // 29522835 / 32803148 = 0.9000
			expectedErr: false,
		},
		{
			name:           "Invalid output - missing Memory usage line",
			output:         `CPU states  :   2.00% user,   1.00% kernel,   97.00% idle\nCurrent memory status: OK`,
			expectedMem:    0,
			expectedErr:    true,
			errDescription: "should fail when Memory usage line is missing",
		},
		{
			name:           "Invalid output - empty string",
			output:         "",
			expectedMem:    0,
			expectedErr:    true,
			errDescription: "should fail for empty output",
		},
		{
			name: "Invalid output - zero total memory",
			output: `CPU states  :   2.00% user,   1.00% kernel,   97.00% idle
Memory usage:   0K total,   0K used,   0K free`,
			expectedMem:    0,
			expectedErr:    true,
			errDescription: "should fail when total memory is zero",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			memUtil, err := parseMemoryUtilization(tt.output, "NX-OS")

			if tt.expectedErr {
				require.Error(t, err, tt.errDescription)
			} else {
				require.NoError(t, err)
				assert.InDelta(t, tt.expectedMem, memUtil, 0.001, "Memory utilization should match")
			}
		})
	}
}

func TestParseMemoryUtilizationIOS(t *testing.T) {
	tests := []struct {
		name           string
		output         string
		expectedMem    float64
		expectedErr    bool
		errDescription string
	}{
		{
			name: "Valid IOS XE output",
			output: `Processor Pool Total:    4096000 Used:     2048000 Free:     2048000
      I/O Pool Total:     512000 Used:      256000 Free:      256000`,
			expectedMem: 0.5, // 2048000 / 4096000 = 0.5
			expectedErr: false,
		},
		{
			name:        "Valid IOS output - high memory usage",
			output:      `Processor Pool Total:   10000000 Used:     9000000 Free:     1000000`,
			expectedMem: 0.9, // 9000000 / 10000000 = 0.9
			expectedErr: false,
		},
		{
			name: "Valid IOS output - low memory usage",
			output: `Processor Pool Total:   10000000 Used:     1000000 Free:     9000000
      I/O Pool Total:    1000000 Used:      100000 Free:      900000`,
			expectedMem: 0.1, // 1000000 / 10000000 = 0.1
			expectedErr: false,
		},
		{
			name:           "Invalid output - missing Processor Pool line",
			output:         `I/O Pool Total:    1000000 Used:      100000 Free:      900000`,
			expectedMem:    0,
			expectedErr:    true,
			errDescription: "should fail when Processor Pool line is missing",
		},
		{
			name:           "Invalid output - empty string",
			output:         "",
			expectedMem:    0,
			expectedErr:    true,
			errDescription: "should fail for empty output",
		},
		{
			name:           "Invalid output - zero total memory",
			output:         `Processor Pool Total:           0 Used:             0 Free:             0`,
			expectedMem:    0,
			expectedErr:    true,
			errDescription: "should fail when total memory is zero",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			memUtil, err := parseMemoryUtilization(tt.output, "IOS")

			if tt.expectedErr {
				require.Error(t, err, tt.errDescription)
			} else {
				require.NoError(t, err)
				assert.InDelta(t, tt.expectedMem, memUtil, 0.001, "Memory utilization should match")
			}
		})
	}
}
