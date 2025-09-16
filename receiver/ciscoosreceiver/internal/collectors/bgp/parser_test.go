// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package bgp

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewParser(t *testing.T) {
	parser := NewParser()
	assert.NotNil(t, parser)
	assert.NotNil(t, parser.neighborPattern)
}

func TestParser_ParseBGPSummary(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		name     string
		input    string
		expected []*Session
		wantErr  bool
	}{
		{
			name: "ios_xe_established_session",
			input: `BGP router identifier 10.0.0.1, local AS number 65000
BGP table version is 1, main routing table version 1

Neighbor        V           AS MsgRcvd MsgSent   TblVer  InQ OutQ Up/Down  State/PfxRcd
10.1.1.1        4        65001     123     456        1    0    0 00:05:23        5`,
			expected: []*Session{
				{
					NeighborIP:       "10.1.1.1",
					ASN:              "65001",
					State:            "Established",
					PrefixesReceived: 5,
					MessagesInput:    123,
					MessagesOutput:   456,
					IsUp:             true,
				},
			},
			wantErr: false,
		},
		{
			name: "nxos_established_sessions",
			input: `BGP router identifier 192.168.1.1, local AS number 65001
BGP table version is 456, Local Router ID is 192.168.1.1
Status codes: s suppressed, d damped, h history, * valid, > best, i - internal

Neighbor        V    AS MsgRcvd MsgSent   TblVer  InQ OutQ Up/Down  State/PfxRcd
192.168.1.2     4 65002    1234    5678        0    0    0 1d02h           25
192.168.1.3     4 65003     500     600        0    0    0 00:30:15         8`,
			expected: []*Session{
				{
					NeighborIP:       "192.168.1.2",
					ASN:              "65002",
					State:            "Established",
					PrefixesReceived: 25,
					MessagesInput:    1234,
					MessagesOutput:   5678,
					IsUp:             true,
				},
				{
					NeighborIP:       "192.168.1.3",
					ASN:              "65003",
					State:            "Established",
					PrefixesReceived: 8,
					MessagesInput:    500,
					MessagesOutput:   600,
					IsUp:             true,
				},
			},
			wantErr: false,
		},
		{
			name: "mixed_session_states",
			input: `BGP router identifier 10.0.0.1, local AS number 65000
BGP table version is 1, main routing table version 1

Neighbor        V           AS MsgRcvd MsgSent   TblVer  InQ OutQ Up/Down  State/PfxRcd
10.1.1.1        4        65001     123     456        1    0    0 00:05:23        5
10.1.1.2        4        65002       0       0        1    0    0    never        Idle
10.1.1.3        4        65003      50      75        1    0    0 01:30:45       10`,
			expected: []*Session{
				{
					NeighborIP:       "10.1.1.1",
					ASN:              "65001",
					State:            "Established",
					PrefixesReceived: 5,
					MessagesInput:    123,
					MessagesOutput:   456,
					IsUp:             true,
				},
				{
					NeighborIP:       "10.1.1.2",
					ASN:              "65002",
					State:            "Idle",
					PrefixesReceived: 0,
					MessagesInput:    0,
					MessagesOutput:   0,
					IsUp:             false,
				},
				{
					NeighborIP:       "10.1.1.3",
					ASN:              "65003",
					State:            "Established",
					PrefixesReceived: 10,
					MessagesInput:    50,
					MessagesOutput:   75,
					IsUp:             true,
				},
			},
			wantErr: false,
		},
		{
			name: "session_down_states",
			input: `BGP router identifier 10.0.0.1, local AS number 65000

Neighbor        V           AS MsgRcvd MsgSent   TblVer  InQ OutQ Up/Down  State/PfxRcd
10.1.1.4        4        65004       5       3        1    0    0 00:00:30      Connect
10.1.1.5        4        65005       2       1        1    0    0 00:00:15       Active
10.1.1.6        4        65006       0       0        1    0    0    never        Idle`,
			expected: []*Session{
				{
					NeighborIP:       "10.1.1.4",
					ASN:              "65004",
					State:            "Idle",
					PrefixesReceived: 0,
					MessagesInput:    5,
					MessagesOutput:   3,
					IsUp:             false,
				},
				{
					NeighborIP:       "10.1.1.5",
					ASN:              "65005",
					State:            "Idle",
					PrefixesReceived: 0,
					MessagesInput:    2,
					MessagesOutput:   1,
					IsUp:             false,
				},
				{
					NeighborIP:       "10.1.1.6",
					ASN:              "65006",
					State:            "Idle",
					PrefixesReceived: 0,
					MessagesInput:    0,
					MessagesOutput:   0,
					IsUp:             false,
				},
			},
			wantErr: false,
		},
		{
			name: "ipv6_neighbors",
			input: `BGP router identifier 10.0.0.1, local AS number 65000
BGP table version is 1, main routing table version 1

Neighbor        V           AS MsgRcvd MsgSent   TblVer  InQ OutQ Up/Down  State/PfxRcd
2001:db8::1     4        65001      50      75        1    0    0 02:15:30        3
fe80::1         4        65002      25      30        1    0    0 01:05:15        1`,
			expected: []*Session{
				{
					NeighborIP:       "2001:db8::1",
					ASN:              "65001",
					State:            "Established",
					PrefixesReceived: 3,
					MessagesInput:    50,
					MessagesOutput:   75,
					IsUp:             true,
				},
				{
					NeighborIP:       "fe80::1",
					ASN:              "65002",
					State:            "Established",
					PrefixesReceived: 1,
					MessagesInput:    25,
					MessagesOutput:   30,
					IsUp:             true,
				},
			},
			wantErr: false,
		},
		{
			name: "zero_prefixes_established",
			input: `BGP router identifier 10.0.0.1, local AS number 65000

Neighbor        V           AS MsgRcvd MsgSent   TblVer  InQ OutQ Up/Down  State/PfxRcd
10.1.1.6        4        65006      10      15        1    0    0 00:10:00        0`,
			expected: []*Session{
				{
					NeighborIP:       "10.1.1.6",
					ASN:              "65006",
					State:            "Established",
					PrefixesReceived: 0,
					MessagesInput:    10,
					MessagesOutput:   15,
					IsUp:             true,
				},
			},
			wantErr: false,
		},
		{
			name:     "empty_output",
			input:    "",
			expected: []*Session{},
			wantErr:  false,
		},
		{
			name: "no_neighbor_lines",
			input: `BGP router identifier 10.0.0.1, local AS number 65000
BGP table version is 1, main routing table version 1

Some other output without neighbor information`,
			expected: []*Session{},
			wantErr:  false,
		},
		{
			name: "malformed_neighbor_line",
			input: `BGP router identifier 10.0.0.1, local AS number 65000

Neighbor        V           AS MsgRcvd MsgSent
10.1.1.7        4        invalid_data`,
			expected: []*Session{},
			wantErr:  false,
		},
		{
			name: "ipv6_neighbor",
			input: `BGP router identifier 10.0.0.1, local AS number 65000

Neighbor        V           AS MsgRcvd MsgSent   TblVer  InQ OutQ Up/Down  State/PfxRcd
2001:db8::1     4        65007      25      30        1    0    0 00:15:00        3`,
			expected: []*Session{
				{
					NeighborIP:       "2001:db8::1",
					ASN:              "65007",
					State:            "Established",
					PrefixesReceived: 3,
					MessagesInput:    25,
					MessagesOutput:   30,
					IsUp:             true,
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sessions, err := parser.ParseBGPSummary(tt.input)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Len(t, sessions, len(tt.expected))

			for i, expected := range tt.expected {
				if i < len(sessions) {
					actual := sessions[i]
					assert.Equal(t, expected.NeighborIP, actual.NeighborIP, "NeighborIP mismatch")
					assert.Equal(t, expected.ASN, actual.ASN, "ASN mismatch")
					assert.Equal(t, expected.State, actual.State, "State mismatch")
					assert.Equal(t, expected.PrefixesReceived, actual.PrefixesReceived, "PrefixesReceived mismatch")
					assert.Equal(t, expected.MessagesInput, actual.MessagesInput, "MessagesInput mismatch")
					assert.Equal(t, expected.MessagesOutput, actual.MessagesOutput, "MessagesOutput mismatch")
					assert.Equal(t, expected.IsUp, actual.IsUp, "IsUp mismatch")
				}
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
			name: "valid_bgp_summary_with_router_id",
			input: `BGP router identifier 10.0.0.1, local AS number 65000
BGP table version is 1, main routing table version 1`,
			expected: true,
		},
		{
			name: "valid_bgp_summary_with_table_version",
			input: `Some output
BGP table version is 5, main routing table version 5
More output`,
			expected: true,
		},
		{
			name: "valid_bgp_summary_with_neighbor_header",
			input: `BGP summary information
Neighbor        V           AS MsgRcvd MsgSent   TblVer  InQ OutQ Up/Down  State/PfxRcd`,
			expected: true,
		},
		{
			name: "valid_bgp_summary_with_as_header",
			input: `BGP information
AS MsgRcvd MsgSent TblVer InQ OutQ Up/Down State/PfxRcd`,
			expected: true,
		},
		{
			name: "invalid_output_no_bgp_indicators",
			input: `This is some random output
without any BGP indicators
just plain text`,
			expected: false,
		},
		{
			name:     "empty_output",
			input:    "",
			expected: false,
		},
		{
			name: "interface_output_not_bgp",
			input: `Interface              IP-Address      OK? Method Status                Protocol
GigabitEthernet0/0     10.1.1.1        YES NVRAM  up                    up`,
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
		"show bgp all summary",
		"show bgp ipv4 unicast summary",
		"show bgp ipv6 unicast summary",
	}

	assert.Equal(t, expectedCommands, commands)
	assert.Len(t, commands, 3)
}

func TestParser_ParseBGPNeighborDetail(t *testing.T) {
	parser := NewParser()

	// Test that the method exists and returns nil (not implemented)
	session, err := parser.ParseBGPNeighborDetail("some output")
	assert.NoError(t, err)
	assert.Nil(t, session)
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
			input: `Neighbor        V           AS MsgRcvd MsgSent   TblVer  InQ OutQ Up/Down  State/PfxRcd
!@#$%^&*()      4        65002       0       0        1    0    0    never        Idle`,
		},
		{
			name: "unicode_characters",
			input: `Neighbor        V           AS MsgRcvd MsgSent   TblVer  InQ OutQ Up/Down  State/PfxRcd
αβγδε           4        65002       0       0        1    0    0    never        Idle`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Should not panic and should return without error
			sessions, err := parser.ParseBGPSummary(tt.input)
			assert.NoError(t, err)
			// Edge cases may return empty slice or nil, both are acceptable
			if sessions != nil {
				assert.Empty(t, sessions)
			}
		})
	}
}
