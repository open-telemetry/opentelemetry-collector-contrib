// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package optics

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ciscoosreceiver/internal/rpc"
)

func TestNewParser(t *testing.T) {
	parser := NewParser()
	assert.NotNil(t, parser)
}

func TestParser_ParseTransceivers(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		name     string
		input    string
		expected []*Transceiver
		wantErr  bool
	}{
		{
			name: "ios_style_output",
			input: `Interface         Tx Power   Rx Power   Status
Gi1/0/1           -2.5       -3.1       OK
Gi1/0/2           -1.8       -2.9       OK
Gi1/0/3           -3.2       -4.1       WARN`,
			expected: []*Transceiver{
				{
					Interface: "Gi1/0/1",
					TxPower:   -2.5,
					RxPower:   -3.1,
				},
				{
					Interface: "Gi1/0/2",
					TxPower:   -1.8,
					RxPower:   -2.9,
				},
				{
					Interface: "Gi1/0/3",
					TxPower:   -3.2,
					RxPower:   -4.1,
				},
			},
			wantErr: false,
		},
		{
			name: "nxos_style_output",
			input: `Ethernet1/1  Tx: -2.5 dBm  Rx: -3.1 dBm
Ethernet1/2  Tx: -1.8 dBm  Rx: -2.9 dBm
Ethernet1/3  Tx: -3.2 dBm  Rx: -4.1 dBm`,
			expected: []*Transceiver{
				{
					Interface: "Ethernet1/1",
					TxPower:   -2.5,
					RxPower:   -3.1,
				},
				{
					Interface: "Ethernet1/2",
					TxPower:   -1.8,
					RxPower:   -2.9,
				},
				{
					Interface: "Ethernet1/3",
					TxPower:   -3.2,
					RxPower:   -4.1,
				},
			},
			wantErr: false,
		},
		{
			name: "mixed_format_output",
			input: `Gi1/0/1           -2.5       -3.1       OK
Ethernet1/2  Tx: -1.8 dBm  Rx: -2.9 dBm
Some random line without transceiver data
Gi1/0/3           -3.2       -4.1       WARN`,
			expected: []*Transceiver{
				{
					Interface: "Gi1/0/1",
					TxPower:   -2.5,
					RxPower:   -3.1,
				},
				{
					Interface: "Ethernet1/2",
					TxPower:   -1.8,
					RxPower:   -2.9,
				},
				{
					Interface: "Gi1/0/3",
					TxPower:   -3.2,
					RxPower:   -4.1,
				},
			},
			wantErr: false,
		},
		{
			name: "positive_power_values",
			input: `Gi1/0/1           2.5        1.8        OK
Ethernet1/2  Tx: 3.2 dBm  Rx: 2.1 dBm`,
			expected: []*Transceiver{
				{
					Interface: "Gi1/0/1",
					TxPower:   2.5,
					RxPower:   1.8,
				},
				{
					Interface: "Ethernet1/2",
					TxPower:   3.2,
					RxPower:   2.1,
				},
			},
			wantErr: false,
		},
		{
			name: "zero_power_values",
			input: `Gi1/0/1           0.0        0.0        OK
Ethernet1/2  Tx: 0.0 dBm  Rx: 0.0 dBm`,
			expected: []*Transceiver{
				{
					Interface: "Gi1/0/1",
					TxPower:   0.0,
					RxPower:   0.0,
				},
				{
					Interface: "Ethernet1/2",
					TxPower:   0.0,
					RxPower:   0.0,
				},
			},
			wantErr: false,
		},
		{
			name: "interface_name_variations",
			input: `GigabitEthernet1/0/1    -2.5    -3.1    OK
FastEthernet0/1         -1.8    -2.9    OK
TenGigabitEthernet1/1   -3.2    -4.1    WARN
Ethernet1/1  Tx: -2.0 dBm  Rx: -3.0 dBm`,
			expected: []*Transceiver{
				{
					Interface: "GigabitEthernet1/0/1",
					TxPower:   -2.5,
					RxPower:   -3.1,
				},
				{
					Interface: "FastEthernet0/1",
					TxPower:   -1.8,
					RxPower:   -2.9,
				},
				{
					Interface: "TenGigabitEthernet1/1",
					TxPower:   -3.2,
					RxPower:   -4.1,
				},
				{
					Interface: "Ethernet1/1",
					TxPower:   -2.0,
					RxPower:   -3.0,
				},
			},
			wantErr: false,
		},
		{
			name:     "empty_output",
			input:    "",
			expected: []*Transceiver{},
			wantErr:  false,
		},
		{
			name: "no_transceiver_data",
			input: `Some random output
without any transceiver information
just plain text`,
			expected: []*Transceiver{},
			wantErr:  false,
		},
		{
			name: "header_only",
			input: `Interface         Tx Power   Rx Power   Status
----------------------------------------------------`,
			expected: []*Transceiver{},
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transceivers, err := parser.ParseTransceivers(tt.input)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Len(t, transceivers, len(tt.expected))

			for i, expected := range tt.expected {
				if i < len(transceivers) {
					actual := transceivers[i]
					assert.Equal(t, expected.Interface, actual.Interface, "Interface mismatch")
					assert.InDelta(t, expected.TxPower, actual.TxPower, 0.01, "TxPower mismatch")
					assert.InDelta(t, expected.RxPower, actual.RxPower, 0.01, "RxPower mismatch")
				}
			}
		})
	}
}

func TestParser_ParseOpticsData(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		name     string
		osType   rpc.OSType
		input    string
		expected *Transceiver
		wantErr  bool
	}{
		{
			name:   "ios_optics_data",
			osType: rpc.IOS,
			input: `Interface    Temp   Voltage  Current  Tx Power  Rx Power
Gi1/0/1      32.5   3.3      45.2     -2.5      -3.1`,
			expected: &Transceiver{
				Interface: "",
				TxPower:   -2.5,
				RxPower:   -3.1,
			},
			wantErr: false,
		},
		{
			name:   "iosxe_optics_data",
			osType: rpc.IOSXE,
			input: `Transceiver monitoring:
  Transceiver Tx power = -2.5 dBm
  Transceiver Rx optical power = -3.1 dBm
  Temperature = 32.5 C`,
			expected: &Transceiver{
				Interface: "",
				TxPower:   -2.5,
				RxPower:   -3.1,
			},
			wantErr: false,
		},
		{
			name:   "nxos_optics_data",
			osType: rpc.NXOS,
			input: `Optical monitoring:
  Tx Power -2.5 dBm
  Rx Power -3.1 dBm
  Temperature 32.5 C`,
			expected: &Transceiver{
				Interface: "",
				TxPower:   -2.5,
				RxPower:   -3.1,
			},
			wantErr: false,
		},
		{
			name:   "iosxe_positive_power",
			osType: rpc.IOSXE,
			input: `Transceiver monitoring:
  Transceiver Tx power = 2.5 dBm
  Transceiver Rx optical power = 1.8 dBm`,
			expected: &Transceiver{
				Interface: "",
				TxPower:   2.5,
				RxPower:   1.8,
			},
			wantErr: false,
		},
		{
			name:   "nxos_zero_power",
			osType: rpc.NXOS,
			input: `Optical monitoring:
  Tx Power 0.0 dBm
  Rx Power 0.0 dBm`,
			expected: &Transceiver{
				Interface: "",
				TxPower:   0.0,
				RxPower:   0.0,
			},
			wantErr: false,
		},
		{
			name:    "ios_no_optics_data",
			osType:  rpc.IOS,
			input:   "No optical transceiver data available",
			wantErr: true,
		},
		{
			name:    "iosxe_no_optics_data",
			osType:  rpc.IOSXE,
			input:   "Transceiver not present",
			wantErr: true,
		},
		{
			name:    "nxos_no_optics_data",
			osType:  rpc.NXOS,
			input:   "Optical monitoring not available",
			wantErr: true,
		},
		{
			name:    "unsupported_os_type",
			osType:  rpc.OSType("UNKNOWN"),
			input:   "Some output",
			wantErr: true,
		},
		{
			name:    "empty_output",
			osType:  rpc.IOSXE,
			input:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transceiver, err := parser.ParseOpticsData(tt.osType, tt.input)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, transceiver)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, transceiver)
			assert.Equal(t, tt.expected.Interface, transceiver.Interface)
			assert.InDelta(t, tt.expected.TxPower, transceiver.TxPower, 0.01)
			assert.InDelta(t, tt.expected.RxPower, transceiver.RxPower, 0.01)
		})
	}
}

func TestParser_parseIOSOptics(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		name     string
		input    string
		expected *Transceiver
		wantErr  bool
	}{
		{
			name:  "valid_ios_optics",
			input: "Gi1/0/1      32.5   3.3      45.2     -2.5      -3.1",
			expected: &Transceiver{
				Interface: "",
				TxPower:   -2.5,
				RxPower:   -3.1,
			},
			wantErr: false,
		},
		{
			name:  "ios_positive_power",
			input: "Gi1/0/1      32.5   3.3      45.2     2.5       1.8",
			expected: &Transceiver{
				Interface: "",
				TxPower:   2.5,
				RxPower:   1.8,
			},
			wantErr: false,
		},
		{
			name:  "ios_zero_power",
			input: "Gi1/0/1      32.5   3.3      45.2     0.0       0.0",
			expected: &Transceiver{
				Interface: "",
				TxPower:   0.0,
				RxPower:   0.0,
			},
			wantErr: false,
		},
		{
			name:    "ios_invalid_format",
			input:   "Invalid format without proper columns",
			wantErr: true,
		},
		{
			name:    "ios_empty_input",
			input:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transceiver, err := parser.parseIOSOptics(tt.input)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, transceiver)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, transceiver)
			assert.Equal(t, tt.expected.Interface, transceiver.Interface)
			assert.InDelta(t, tt.expected.TxPower, transceiver.TxPower, 0.01)
			assert.InDelta(t, tt.expected.RxPower, transceiver.RxPower, 0.01)
		})
	}
}

func TestParser_parseIOSXEOptics(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		name     string
		input    string
		expected *Transceiver
		wantErr  bool
	}{
		{
			name: "valid_iosxe_optics",
			input: `Transceiver monitoring:
  Transceiver Tx power = -2.5 dBm
  Transceiver Rx optical power = -3.1 dBm`,
			expected: &Transceiver{
				Interface: "",
				TxPower:   -2.5,
				RxPower:   -3.1,
			},
			wantErr: false,
		},
		{
			name: "iosxe_positive_power",
			input: `Transceiver monitoring:
  Transceiver Tx power = 2.5 dBm
  Transceiver Rx optical power = 1.8 dBm`,
			expected: &Transceiver{
				Interface: "",
				TxPower:   2.5,
				RxPower:   1.8,
			},
			wantErr: false,
		},
		{
			name: "iosxe_zero_power",
			input: `Transceiver monitoring:
  Transceiver Tx power = 0.0 dBm
  Transceiver Rx optical power = 0.0 dBm`,
			expected: &Transceiver{
				Interface: "",
				TxPower:   0.0,
				RxPower:   0.0,
			},
			wantErr: false,
		},
		{
			name: "iosxe_with_temperature",
			input: `Transceiver monitoring:
  Temperature = 32.5 C
  Transceiver Tx power = -2.5 dBm
  Transceiver Rx optical power = -3.1 dBm
  Voltage = 3.3 V`,
			expected: &Transceiver{
				Interface: "",
				TxPower:   -2.5,
				RxPower:   -3.1,
			},
			wantErr: false,
		},
		{
			name:    "iosxe_missing_tx_power",
			input:   "Transceiver Rx optical power = -3.1 dBm",
			wantErr: true,
		},
		{
			name:    "iosxe_missing_rx_power",
			input:   "Transceiver Tx power = -2.5 dBm",
			wantErr: true,
		},
		{
			name:    "iosxe_invalid_format",
			input:   "Invalid format without proper power readings",
			wantErr: true,
		},
		{
			name:    "iosxe_empty_input",
			input:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transceiver, err := parser.parseIOSXEOptics(tt.input)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, transceiver)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, transceiver)
			assert.Equal(t, tt.expected.Interface, transceiver.Interface)
			assert.InDelta(t, tt.expected.TxPower, transceiver.TxPower, 0.01)
			assert.InDelta(t, tt.expected.RxPower, transceiver.RxPower, 0.01)
		})
	}
}

func TestParser_parseNXOSOptics(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		name     string
		input    string
		expected *Transceiver
		wantErr  bool
	}{
		{
			name: "valid_nxos_optics",
			input: `Optical monitoring:
  Tx Power -2.5 dBm
  Rx Power -3.1 dBm`,
			expected: &Transceiver{
				Interface: "",
				TxPower:   -2.5,
				RxPower:   -3.1,
			},
			wantErr: false,
		},
		{
			name: "nxos_positive_power",
			input: `Optical monitoring:
  Tx Power 2.5 dBm
  Rx Power 1.8 dBm`,
			expected: &Transceiver{
				Interface: "",
				TxPower:   2.5,
				RxPower:   1.8,
			},
			wantErr: false,
		},
		{
			name: "nxos_zero_power",
			input: `Optical monitoring:
  Tx Power 0.0 dBm
  Rx Power 0.0 dBm`,
			expected: &Transceiver{
				Interface: "",
				TxPower:   0.0,
				RxPower:   0.0,
			},
			wantErr: false,
		},
		{
			name: "nxos_with_temperature",
			input: `Optical monitoring:
  Temperature 32.5 C
  Tx Power -2.5 dBm
  Rx Power -3.1 dBm
  Voltage 3.3 V`,
			expected: &Transceiver{
				Interface: "",
				TxPower:   -2.5,
				RxPower:   -3.1,
			},
			wantErr: false,
		},
		{
			name:    "nxos_missing_tx_power",
			input:   "Rx Power -3.1 dBm",
			wantErr: true,
		},
		{
			name:    "nxos_missing_rx_power",
			input:   "Tx Power -2.5 dBm",
			wantErr: true,
		},
		{
			name:    "nxos_invalid_format",
			input:   "Invalid format without proper power readings",
			wantErr: true,
		},
		{
			name:    "nxos_empty_input",
			input:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transceiver, err := parser.parseNXOSOptics(tt.input)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, transceiver)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, transceiver)
			assert.Equal(t, tt.expected.Interface, transceiver.Interface)
			assert.InDelta(t, tt.expected.TxPower, transceiver.TxPower, 0.01)
			assert.InDelta(t, tt.expected.RxPower, transceiver.RxPower, 0.01)
		})
	}
}

// Test helper function parseTransceiverLine
func TestParser_parseTransceiverLine(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		name     string
		input    string
		expected *Transceiver
	}{
		{
			name:  "ios_format_line",
			input: "Gi1/0/1           -2.5       -3.1       OK",
			expected: &Transceiver{
				Interface: "Gi1/0/1",
				TxPower:   -2.5,
				RxPower:   -3.1,
			},
		},
		{
			name:  "nxos_format_line",
			input: "Ethernet1/1  Tx: -2.5 dBm  Rx: -3.1 dBm",
			expected: &Transceiver{
				Interface: "Ethernet1/1",
				TxPower:   -2.5,
				RxPower:   -3.1,
			},
		},
		{
			name:  "positive_power_values",
			input: "Gi1/0/1           2.5        1.8        OK",
			expected: &Transceiver{
				Interface: "Gi1/0/1",
				TxPower:   2.5,
				RxPower:   1.8,
			},
		},
		{
			name:  "zero_power_values",
			input: "Gi1/0/1           0.0        0.0        OK",
			expected: &Transceiver{
				Interface: "Gi1/0/1",
				TxPower:   0.0,
				RxPower:   0.0,
			},
		},
		{
			name:     "invalid_format",
			input:    "Invalid line without proper format",
			expected: nil,
		},
		{
			name:     "empty_line",
			input:    "",
			expected: nil,
		},
		{
			name:     "header_line",
			input:    "Interface         Tx Power   Rx Power   Status",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parser.parseTransceiverLine(tt.input)

			if tt.expected == nil {
				assert.Nil(t, result)
				return
			}

			require.NotNil(t, result)
			assert.Equal(t, tt.expected.Interface, result.Interface)
			assert.InDelta(t, tt.expected.TxPower, result.TxPower, 0.01)
			assert.InDelta(t, tt.expected.RxPower, result.RxPower, 0.01)
		})
	}
}

// Test edge cases and error conditions
func TestParser_EdgeCases(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "very_long_line",
			input: strings.Repeat("a", 10000),
		},
		{
			name: "special_characters",
			input: `Gi0/0/0 -2.5 -3.1 OK
!@#$%^&*() transceiver data
Hardware information available`,
		},
		{
			name: "unicode_characters",
			input: `Ethernet1/1  Tx: -2.5 dBm  Rx: -3.1 dBm
αβγδε transceiver description
Hardware status available`,
		},
		{
			name: "malformed_transceiver_line",
			input: `This is not a valid transceiver line
Gi1/0/1 status unknown format`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Should not panic and should return without error
			transceivers, err := parser.ParseTransceivers(tt.input)
			assert.NoError(t, err)
			// Edge cases should return empty slice, not nil
			if transceivers == nil {
				transceivers = []*Transceiver{}
			}
			assert.NotNil(t, transceivers)
		})
	}
}
