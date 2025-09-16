// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package interfaces

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewParser(t *testing.T) {
	parser := NewParser()
	assert.NotNil(t, parser)
}

func TestParser_ParseInterfaces(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		name     string
		input    string
		expected []*Interface
		wantErr  bool
	}{
		{
			name: "ios_xe_interface_output",
			input: `GigabitEthernet0/0/0 is up, line protocol is up
  Hardware is GigE, address is 0012.7f57.ac02 (bia 0012.7f57.ac02)
  Description: Connection to Core Switch
  Internet address is 10.1.1.1/24
  MTU 1500 bytes, BW 1000000 Kbit/sec, DLY 10 usec,
     reliability 255/255, txload 1/255, rxload 1/255
  Encapsulation ARPA, loopback not set
  Keepalive set (10 sec)
  Full-duplex, 1000Mb/s, media type is T
  input flow-control is off, output flow-control is unsupported
  ARP type: ARPA, ARP Timeout 04:00:00
  Last input 00:00:08, output 00:00:05, output hang never
  Last clearing of "show interface" counters never
  Input queue: 0/75/0/0 (size/max/drops/flushes); Total output drops: 0
  Queueing strategy: fifo
  Output queue: 0/40 (size/max)
  5 minute input rate 1000 bits/sec, 2 packets/sec
  5 minute output rate 2000 bits/sec, 3 packets/sec
     1548 packets input, 193536 bytes, 0 no buffer
     Received 1200 broadcasts (800 IP multicast)
     0 runts, 0 giants, 0 throttles
     0 input errors, 0 CRC, 0 frame, 0 overrun, 0 ignored
     0 watchdog, 800 multicast, 0 pause input
     1789 packets output, 243840 bytes, 0 underruns
     0 output errors, 0 collisions, 2 interface resets
     0 unknown protocol drops
     0 babbles, 0 late collision, 0 deferred
     0 lost carrier, 0 no carrier, 0 pause output
     0 output buffer failures, 0 output buffers swapped out`,
			expected: []*Interface{
				{
					Name:           "GigabitEthernet0/0/0",
					AdminStatus:    StatusUp,
					OperStatus:     StatusUp,
					Description:    "Connection to Core Switch",
					MACAddress:     "0012.7f57.ac02",
					InputBytes:     193536.0,
					OutputBytes:    243840.0,
					InputErrors:    0.0,
					OutputErrors:   0.0,
					InputDrops:     0.0,
					OutputDrops:    0.0,
					InputBroadcast: 1200.0,
					InputMulticast: 800.0,
					Speed:          1000,
					SpeedString:    "1000 Mb/s",
				},
			},
			wantErr: false,
		},
		{
			name: "nx_os_interface_output_disabled",
			input: `Ethernet1/1 is up
admin state is up, Dedicated Interface
  Hardware: 1000/10000 Ethernet, address: 0050.5682.7b8a (bia 0050.5682.7b8a)
  Description: Uplink to Distribution
  MTU 1500 bytes, BW 10000000 Kbit
  reliability 255/255, txload 1/255, rxload 1/255
  Encapsulation ARPA
  Port mode is trunk
  full-duplex, 10 Gb/s
  Beacon is turned off
  Auto-Negotiation is turned on  FEC mode is Auto
  Input flow-control is off, output flow-control is off
  Auto-mdix is turned off
  Switchport monitor is off
  EtherType is 0x8100
  Last link flapped 00:10:15
  Last clearing of "show interface" counters never
  30 seconds input rate 8 bits/sec, 0 packets/sec
  30 seconds output rate 16 bits/sec, 0 packets/sec
  Load-Interval #2: 5 minute (300 seconds)
    input rate 12 bps, 0 pps; output rate 24 bps, 0 pps
  RX
    2500 unicast packets  1500 multicast packets  800 broadcast packets
    4800 input packets  614400 bytes
    0 jumbo packets  0 storm suppression packets
    0 runts  0 giants  0 CRC  0 no buffer
    0 input error  0 short frame  0 overrun   0 underrun  0 ignored
    0 watchdog  0 bad etype drop  0 bad proto drop  0 if down drop
    0 input with dribble  0 input discard
    0 Rx pause
  TX
    3200 unicast packets  200 multicast packets  100 broadcast packets
    3500 output packets  448000 bytes
    0 jumbo packets
    0 output error  0 collision  0 deferred  0 late collision
    0 lost carrier  0 no carrier  0 babble  0 output discard
    0 Tx pause`,
			expected: []*Interface{
				{
					Name:           "Ethernet1/1",
					AdminStatus:    StatusUp,
					OperStatus:     StatusUp,
					Description:    "Uplink to Distribution",
					MACAddress:     "0050.5682.7b8a",
					InputBytes:     614400.0,
					OutputBytes:    448000.0,
					InputErrors:    0.0,
					OutputErrors:   0.0,
					InputMulticast: 1500.0,
					InputBroadcast: 800.0,
					Speed:          0,
					SpeedString:    "",
				},
			},
			wantErr: false,
		},
		{
			name: "interface_administratively_down",
			input: `GigabitEthernet0/0/1 is administratively down, line protocol is down
  Hardware is GigE, address is 0012.7f57.ac03 (bia 0012.7f57.ac03)
  MTU 1500 bytes, BW 1000000 Kbit/sec, DLY 10 usec,
     reliability 255/255, txload 1/255, rxload 1/255
  Encapsulation ARPA, loopback not set
  Keepalive set (10 sec)
  Full-duplex, 1000Mb/s, media type is T
  input flow-control is off, output flow-control is unsupported
  ARP type: ARPA, ARP Timeout 04:00:00
  Last input never, output never, output hang never
  Last clearing of "show interface" counters never
  Input queue: 0/75/0/0 (size/max/drops/flushes); Total output drops: 0
  Queueing strategy: fifo
  Output queue: 0/40 (size/max)
  5 minute input rate 0 bits/sec, 0 packets/sec
  5 minute output rate 0 bits/sec, 0 packets/sec
     0 packets input, 0 bytes, 0 no buffer
     Received 0 broadcasts (0 IP multicast)
     0 runts, 0 giants, 0 throttles
     0 input errors, 0 CRC, 0 frame, 0 overrun, 0 ignored
     0 watchdog, 0 multicast, 0 pause input
     0 packets output, 0 bytes, 0 underruns
     0 output errors, 0 collisions, 0 interface resets
     0 unknown protocol drops
     0 babbles, 0 late collision, 0 deferred
     0 lost carrier, 0 no carrier, 0 pause output
     0 output buffer failures, 0 output buffers swapped out`,
			expected: []*Interface{
				{
					Name:           "GigabitEthernet0/0/1",
					AdminStatus:    StatusDown,
					OperStatus:     StatusDown,
					MACAddress:     "0012.7f57.ac03",
					InputBytes:     0.0,
					OutputBytes:    0.0,
					InputErrors:    0.0,
					OutputErrors:   0.0,
					InputDrops:     0.0,
					OutputDrops:    0.0,
					InputBroadcast: 0.0,
					InputMulticast: 0.0,
					Speed:          1000,
					SpeedString:    "1000 Mb/s",
				},
			},
			wantErr: false,
		},
		{
			name: "multiple_interfaces",
			input: `GigabitEthernet0/0/0 is up, line protocol is up
  Hardware is GigE, address is 0012.7f57.ac02 (bia 0012.7f57.ac02)
  Description: Uplink
  Full-duplex, 1000Mb/s, media type is T
  Input queue: 0/75/0/0 (size/max/drops/flushes); Total output drops: 0
     100 packets input, 12800 bytes, 0 no buffer
     0 input errors, 0 CRC, 0 frame, 0 overrun, 0 ignored
     200 packets output, 25600 bytes, 0 underruns
     0 output errors, 0 collisions, 0 interface resets

GigabitEthernet0/0/1 is down, line protocol is down
  Hardware is GigE, address is 0012.7f57.ac03 (bia 0012.7f57.ac03)
  Full-duplex, 100Mb/s, media type is T
  Input queue: 0/75/5/0 (size/max/drops/flushes); Total output drops: 2
     0 packets input, 0 bytes, 0 no buffer
     1 input errors, 0 CRC, 0 frame, 0 overrun, 0 ignored
     0 packets output, 0 bytes, 0 underruns
     3 output errors, 0 collisions, 0 interface resets`,
			expected: []*Interface{
				{
					Name:         "GigabitEthernet0/0/0",
					AdminStatus:  StatusUp,
					OperStatus:   StatusUp,
					Description:  "Uplink",
					MACAddress:   "0012.7f57.ac02",
					InputBytes:   12800.0,
					OutputBytes:  25600.0,
					InputErrors:  0.0,
					OutputErrors: 0.0,
					InputDrops:   0.0,
					OutputDrops:  0.0,
					Speed:        1000,
					SpeedString:  "1000 Mb/s",
				},
				{
					Name:         "GigabitEthernet0/0/1",
					AdminStatus:  StatusUp,
					OperStatus:   StatusDown,
					MACAddress:   "0012.7f57.ac03",
					InputBytes:   0.0,
					OutputBytes:  0.0,
					InputErrors:  1.0,
					OutputErrors: 3.0,
					InputDrops:   5.0,
					OutputDrops:  2.0,
					Speed:        100,
					SpeedString:  "100 Mb/s",
				},
			},
			wantErr: false,
		},
		{
			name: "vlan_interface",
			input: `Vlan100 is up, line protocol is up
  Hardware is EtherSVI, address is 0012.7f57.ac04 (bia 0012.7f57.ac04)
  Description: Management VLAN
  Internet address is 192.168.100.1/24
  MTU 1500 bytes, BW 1000000 Kbit/sec, DLY 10 usec,
     reliability 255/255, txload 1/255, rxload 1/255
  Encapsulation ARPA, loopback not set
  Keepalive not supported
  ARP type: ARPA, ARP Timeout 04:00:00
  Last input 00:00:01, output 00:00:01, output hang never
  Last clearing of "show interface" counters never
  Input queue: 0/75/0/0 (size/max/drops/flushes); Total output drops: 0
  Queueing strategy: fifo
  Output queue: 0/40 (size/max)
  5 minute input rate 500 bits/sec, 1 packets/sec
  5 minute output rate 600 bits/sec, 1 packets/sec
     50 packets input, 6400 bytes
     Received 30 broadcasts (20 IP multicast)
     0 runts, 0 giants, 0 throttles
     0 input errors, 0 CRC, 0 frame, 0 overrun, 0 ignored
     60 packets output, 7680 bytes, 0 underruns
     0 output errors, 0 interface resets
     0 unknown protocol drops`,
			expected: []*Interface{
				{
					Name:           "Vlan100",
					AdminStatus:    StatusUp,
					OperStatus:     StatusUp,
					Description:    "Management VLAN",
					MACAddress:     "0012.7f57.ac04",
					InputBytes:     6400.0,
					OutputBytes:    7680.0,
					InputErrors:    0.0,
					OutputErrors:   0.0,
					InputDrops:     0.0,
					OutputDrops:    0.0,
					InputBroadcast: 30.0,
					InputMulticast: 20.0,
				},
			},
			wantErr: false,
		},
		{
			name: "cisco_exporter_asr_1000_real_output",
			input: `GigabitEthernet0/0/0 is up, line protocol is up
  Hardware is ASR1000-SIP-10, address is 70b3.17ff.6500 (bia 70b3.17ff.6500)
  Description: WAN Interface
  Internet address is 192.168.1.1/30
  MTU 1500 bytes, BW 1000000 Kbit/sec, DLY 10 usec,
     reliability 255/255, txload 1/255, rxload 1/255
  Encapsulation ARPA, loopback not set
  Keepalive set (10 sec)
  Full-duplex, 1000Mb/s, link type is auto, media type is T
  output flow-control is unsupported, input flow-control is unsupported
  ARP type: ARPA, ARP Timeout 04:00:00
  Last input 00:00:00, output 00:00:00, output hang never
  Last clearing of "show interface" counters 1w0d
  Input queue: 0/375/0/0 (size/max/drops/flushes); Total output drops: 0
  Queueing strategy: fifo
  Output queue: 0/40 (size/max)
  5 minute input rate 2000 bits/sec, 4 packets/sec
  5 minute output rate 3000 bits/sec, 5 packets/sec
     2847392 packets input, 364226816 bytes, 0 no buffer
     Received 1847 broadcasts (0 IP multicast)
     0 runts, 0 giants, 0 throttles
     0 input errors, 0 CRC, 0 frame, 0 overrun, 0 ignored
     0 watchdog, 1847 multicast, 0 pause input
     2847393 packets output, 364226817 bytes, 0 underruns
     0 output errors, 0 collisions, 1 interface resets
     0 unknown protocol drops
     0 babbles, 0 late collision, 0 deferred
     0 lost carrier, 0 no carrier, 0 pause output
     0 output buffer failures, 0 output buffers swapped out

TenGigabitEthernet0/1/0 is up, line protocol is up
  Hardware is ASR1000-SIP-40, address is 70b3.17ff.6501 (bia 70b3.17ff.6501)
  Description: Core Uplink
  MTU 1500 bytes, BW 10000000 Kbit/sec, DLY 10 usec,
     reliability 255/255, txload 1/255, rxload 1/255
  Encapsulation ARPA, loopback not set
  Keepalive set (10 sec)
  Full-duplex, 10000Mb/s, link type is auto, media type is LR
  output flow-control is unsupported, input flow-control is unsupported
  ARP type: ARPA, ARP Timeout 04:00:00
  Last input 00:00:00, output 00:00:00, output hang never
  Last clearing of "show interface" counters never
  Input queue: 0/2000/0/0 (size/max/drops/flushes); Total output drops: 0
  Queueing strategy: fifo
  Output queue: 0/40 (size/max)
  5 minute input rate 50000 bits/sec, 75 packets/sec
  5 minute output rate 60000 bits/sec, 85 packets/sec
     45847392 packets input, 5864226816 bytes, 0 no buffer
     Received 18470 broadcasts (5000 IP multicast)
     0 runts, 0 giants, 0 throttles
     0 input errors, 0 CRC, 0 frame, 0 overrun, 0 ignored
     0 watchdog, 5000 multicast, 0 pause input
     45847393 packets output, 5864226817 bytes, 0 underruns
     0 output errors, 0 collisions, 0 interface resets
     0 unknown protocol drops
     0 babbles, 0 late collision, 0 deferred
     0 lost carrier, 0 no carrier, 0 pause output
     0 output buffer failures, 0 output buffers swapped out`,
			expected: []*Interface{
				{
					Name:           "GigabitEthernet0/0/0",
					AdminStatus:    StatusUp,
					OperStatus:     StatusUp,
					Description:    "WAN Interface",
					MACAddress:     "70b3.17ff.6500",
					InputBytes:     364226816.0,
					OutputBytes:    364226817.0,
					InputErrors:    0.0,
					OutputErrors:   0.0,
					InputDrops:     0.0,
					OutputDrops:    0.0,
					InputBroadcast: 1847.0,
					InputMulticast: 0.0,
					Speed:          1000,
					SpeedString:    "1000 Mb/s",
				},
				{
					Name:           "TenGigabitEthernet0/1/0",
					AdminStatus:    StatusUp,
					OperStatus:     StatusUp,
					Description:    "Core Uplink",
					MACAddress:     "70b3.17ff.6501",
					InputBytes:     5864226816.0,
					OutputBytes:    5864226817.0,
					InputErrors:    0.0,
					OutputErrors:   0.0,
					InputDrops:     0.0,
					OutputDrops:    0.0,
					InputBroadcast: 18470.0,
					InputMulticast: 5000.0,
					Speed:          10000,
					SpeedString:    "10000 Mb/s",
				},
			},
			wantErr: false,
		},
		{
			name: "cisco_exporter_catalyst_real_output_with_errors",
			input: `GigabitEthernet1/0/1 is up, line protocol is up (connected)
  Hardware is Gigabit Ethernet, address is 001e.7a3f.8c01 (bia 001e.7a3f.8c01)
  Description: User Port
  MTU 1500 bytes, BW 1000000 Kbit/sec, DLY 10 usec,
     reliability 255/255, txload 1/255, rxload 1/255
  Encapsulation ARPA, loopback not set
  Keepalive set (10 sec)
  Full-duplex, 1000Mb/s, media type is 10/100/1000BaseTX
  input flow-control is off, output flow-control is unsupported
  ARP type: ARPA, ARP Timeout 04:00:00
  Last input 00:00:05, output 00:00:01, output hang never
  Last clearing of "show interface" counters never
  Input queue: 0/75/25/0 (size/max/drops/flushes); Total output drops: 15
  Queueing strategy: fifo
  Output queue: 0/40 (size/max)
  5 minute input rate 1500 bits/sec, 3 packets/sec
  5 minute output rate 2500 bits/sec, 4 packets/sec
     1847392 packets input, 236226816 bytes, 0 no buffer
     Received 18470 broadcasts (12000 IP multicast)
     0 runts, 0 giants, 0 throttles
     5 input errors, 2 CRC, 1 frame, 1 overrun, 1 ignored
     0 watchdog, 12000 multicast, 0 pause input
     1847393 packets output, 236226817 bytes, 0 underruns
     3 output errors, 1 collisions, 0 interface resets
     0 unknown protocol drops
     0 babbles, 1 late collision, 1 deferred
     0 lost carrier, 0 no carrier, 0 pause output
     0 output buffer failures, 0 output buffers swapped out`,
			expected: []*Interface{
				{
					Name:           "GigabitEthernet1/0/1",
					AdminStatus:    StatusUp,
					OperStatus:     StatusUp,
					Description:    "User Port",
					MACAddress:     "001e.7a3f.8c01",
					InputBytes:     236226816.0,
					OutputBytes:    236226817.0,
					InputErrors:    5.0,
					OutputErrors:   3.0,
					InputDrops:     25.0,
					OutputDrops:    15.0,
					InputBroadcast: 18470.0,
					InputMulticast: 12000.0,
					Speed:          1000,
					SpeedString:    "1000 Mb/s",
				},
			},
			wantErr: false,
		},
		{
			name: "no_interface_data",
			input: `Some random output
without interface information
just plain text`,
			expected: []*Interface{},
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			interfaces, err := parser.ParseInterfaces(tt.input)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Len(t, interfaces, len(tt.expected))

			for i, expected := range tt.expected {
				if i < len(interfaces) {
					actual := interfaces[i]
					assert.Equal(t, expected.Name, actual.Name, "Name mismatch")
					assert.Equal(t, expected.AdminStatus, actual.AdminStatus, "AdminStatus mismatch")
					assert.Equal(t, expected.OperStatus, actual.OperStatus, "OperStatus mismatch")
					assert.Equal(t, expected.Description, actual.Description, "Description mismatch")
					assert.Equal(t, expected.MACAddress, actual.MACAddress, "MACAddress mismatch")
					assert.Equal(t, expected.InputBytes, actual.InputBytes, "InputBytes mismatch")
					assert.Equal(t, expected.OutputBytes, actual.OutputBytes, "OutputBytes mismatch")
					assert.Equal(t, expected.InputErrors, actual.InputErrors, "InputErrors mismatch")
					assert.Equal(t, expected.OutputErrors, actual.OutputErrors, "OutputErrors mismatch")
					assert.Equal(t, expected.InputDrops, actual.InputDrops, "InputDrops mismatch")
					assert.Equal(t, expected.OutputDrops, actual.OutputDrops, "OutputDrops mismatch")
					assert.Equal(t, expected.InputBroadcast, actual.InputBroadcast, "InputBroadcast mismatch")
					assert.Equal(t, expected.InputMulticast, actual.InputMulticast, "InputMulticast mismatch")
					assert.Equal(t, expected.Speed, actual.Speed, "Speed mismatch")
					assert.Equal(t, expected.SpeedString, actual.SpeedString, "SpeedString mismatch")
				}
			}
		})
	}
}

func TestParser_ParseSimpleInterfaces(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		name     string
		input    string
		expected []*Interface
		wantErr  bool
	}{
		{
			name: "simple_interface_status",
			input: `GigabitEthernet0/0/0 is up, line protocol is up
GigabitEthernet0/0/1 is down, line protocol is down
Vlan100 is up, line protocol is up`,
			expected: []*Interface{
				{
					Name:        "GigabitEthernet0/0/0",
					AdminStatus: StatusUp,
					OperStatus:  StatusUp,
				},
				{
					Name:        "GigabitEthernet0/0/1",
					AdminStatus: StatusUp,
					OperStatus:  StatusDown,
				},
				{
					Name:        "Vlan100",
					AdminStatus: StatusUp,
					OperStatus:  StatusUp,
				},
			},
			wantErr: false,
		},
		{
			name: "interface_without_line_protocol",
			input: `mgmt0 is up
Ethernet1/1 is down`,
			expected: []*Interface{
				{
					Name:        "mgmt0",
					AdminStatus: StatusUp,
					OperStatus:  StatusUp,
				},
				{
					Name:        "Ethernet1/1",
					AdminStatus: StatusUp,
					OperStatus:  StatusDown,
				},
			},
			wantErr: false,
		},
		{
			name: "cisco_exporter_ios_xe_real_device_output",
			input: `GigabitEthernet1/0/1 is up, line protocol is up
GigabitEthernet1/0/2 is up, line protocol is up
GigabitEthernet1/0/3 is down, line protocol is down
GigabitEthernet1/0/4 is administratively down, line protocol is down
TenGigabitEthernet1/0/1 is up, line protocol is up
TenGigabitEthernet1/0/2 is down, line protocol is down
Vlan1 is up, line protocol is up
Vlan100 is up, line protocol is up
Loopback0 is up, line protocol is up`,
			expected: []*Interface{
				{Name: "GigabitEthernet1/0/1", AdminStatus: StatusUp, OperStatus: StatusUp},
				{Name: "GigabitEthernet1/0/2", AdminStatus: StatusUp, OperStatus: StatusUp},
				{Name: "GigabitEthernet1/0/3", AdminStatus: StatusUp, OperStatus: StatusDown},
				{Name: "GigabitEthernet1/0/4", AdminStatus: StatusDown, OperStatus: StatusDown},
				{Name: "TenGigabitEthernet1/0/1", AdminStatus: StatusUp, OperStatus: StatusUp},
				{Name: "TenGigabitEthernet1/0/2", AdminStatus: StatusUp, OperStatus: StatusDown},
				{Name: "Vlan1", AdminStatus: StatusUp, OperStatus: StatusUp},
				{Name: "Vlan100", AdminStatus: StatusUp, OperStatus: StatusUp},
				{Name: "Loopback0", AdminStatus: StatusUp, OperStatus: StatusUp},
			},
			wantErr: false,
		},
		{
			name: "cisco_exporter_nx_os_real_device_output",
			input: `Ethernet1/1 is up
Ethernet1/2 is up
Ethernet1/3 is down (Link not connected)
Ethernet1/4 is down (Administratively down)
Ethernet1/5 is up
mgmt0 is up
Vlan1 is up
Vlan100 is up
port-channel1 is up`,
			expected: []*Interface{
				{Name: "Ethernet1/1", AdminStatus: StatusUp, OperStatus: StatusUp},
				{Name: "Ethernet1/2", AdminStatus: StatusUp, OperStatus: StatusUp},
				{Name: "Ethernet1/3", AdminStatus: StatusUp, OperStatus: StatusDown},
				{Name: "Ethernet1/4", AdminStatus: StatusDown, OperStatus: StatusDown},
				{Name: "Ethernet1/5", AdminStatus: StatusUp, OperStatus: StatusUp},
				{Name: "mgmt0", AdminStatus: StatusUp, OperStatus: StatusUp},
				{Name: "Vlan1", AdminStatus: StatusUp, OperStatus: StatusUp},
				{Name: "Vlan100", AdminStatus: StatusUp, OperStatus: StatusUp},
				{Name: "port-channel1", AdminStatus: StatusUp, OperStatus: StatusUp},
			},
			wantErr: false,
		},
		{
			name: "cisco_exporter_ios_real_device_output",
			input: `FastEthernet0/1 is up, line protocol is up
FastEthernet0/2 is down, line protocol is down
FastEthernet0/3 is administratively down, line protocol is down
Serial0/0/0 is up, line protocol is up
Serial0/0/1 is down, line protocol is down
Loopback0 is up, line protocol is up
Tunnel0 is up, line protocol is up`,
			expected: []*Interface{
				{Name: "FastEthernet0/1", AdminStatus: StatusUp, OperStatus: StatusUp},
				{Name: "FastEthernet0/2", AdminStatus: StatusUp, OperStatus: StatusDown},
				{Name: "FastEthernet0/3", AdminStatus: StatusDown, OperStatus: StatusDown},
				{Name: "Serial0/0/0", AdminStatus: StatusUp, OperStatus: StatusUp},
				{Name: "Serial0/0/1", AdminStatus: StatusUp, OperStatus: StatusDown},
				{Name: "Loopback0", AdminStatus: StatusUp, OperStatus: StatusUp},
				{Name: "Tunnel0", AdminStatus: StatusUp, OperStatus: StatusUp},
			},
			wantErr: false,
		},
		{
			name:     "no_interface_matches",
			input:    "Some random text without interface status",
			expected: []*Interface{},
			wantErr:  false,
		},
		{
			name:     "empty_output",
			input:    "",
			expected: []*Interface{},
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			interfaces, err := parser.ParseSimpleInterfaces(tt.input)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Len(t, interfaces, len(tt.expected))

			for i, expected := range tt.expected {
				if i < len(interfaces) {
					actual := interfaces[i]
					assert.Equal(t, expected.Name, actual.Name)
					assert.Equal(t, expected.AdminStatus, actual.AdminStatus)
					assert.Equal(t, expected.OperStatus, actual.OperStatus)
				}
			}
		})
	}
}

func TestParser_ParseVLANs(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		name       string
		vlanOutput string
		interfaces []*Interface
		expected   map[string][]string // interface name -> VLANs
	}{
		{
			name: "vlan_association",
			vlanOutput: `VLAN Name                             Status    Ports
---- -------------------------------- --------- -------------------------------
1    default                          active    Gi0/0/1, Gi0/0/2
100  Management                       active    Gi0/0/3
200  Guest                            active    Gi0/0/4, Gi0/0/5`,
			interfaces: []*Interface{
				NewInterface("Gi0/0/1"),
				NewInterface("Gi0/0/2"),
				NewInterface("Gi0/0/3"),
				NewInterface("Gi0/0/4"),
			},
			expected: map[string][]string{
				"Gi0/0/1": {"1"},
				"Gi0/0/2": {"1"},
				"Gi0/0/3": {"100"},
				"Gi0/0/4": {"200"},
			},
		},
		{
			name: "vlan_id_in_description",
			vlanOutput: `Interface Gi0/0/1
  Encapsulation 802.1Q Virtual LAN, Vlan ID  100
Interface Gi0/0/2
  Encapsulation 802.1Q Virtual LAN, Vlan ID  200`,
			interfaces: []*Interface{
				NewInterface("Gi0/0/1"),
				NewInterface("Gi0/0/2"),
			},
			expected: map[string][]string{
				"Gi0/0/1": {"100"},
				"Gi0/0/2": {"200"},
			},
		},
		{
			name:       "no_vlan_info",
			vlanOutput: "No VLAN information available",
			interfaces: []*Interface{
				NewInterface("Gi0/0/1"),
			},
			expected: map[string][]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser.ParseVLANs(tt.vlanOutput, tt.interfaces)

			for ifaceName, expectedVLANs := range tt.expected {
				var foundInterface *Interface
				for _, iface := range tt.interfaces {
					if iface.Name == ifaceName {
						foundInterface = iface
						break
					}
				}

				require.NotNil(t, foundInterface, "Interface %s not found", ifaceName)
				assert.Equal(t, expectedVLANs, foundInterface.VLANs, "VLAN mismatch for interface %s", ifaceName)
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
			name: "valid_interface_output",
			input: `GigabitEthernet0/0/0 is up, line protocol is up
Interface information available`,
			expected: true,
		},
		{
			name: "valid_gigabitethernet_output",
			input: `GigabitEthernet1/0/1 status
Hardware information`,
			expected: true,
		},
		{
			name: "valid_fastethernet_output",
			input: `FastEthernet0/1 configuration
Port details available`,
			expected: true,
		},
		{
			name: "valid_ethernet_output",
			input: `Ethernet1/1 is operational
Link status information`,
			expected: true,
		},
		{
			name: "valid_line_protocol_output",
			input: `Interface status:
line protocol is up`,
			expected: true,
		},
		{
			name: "valid_is_up_output",
			input: `Port status:
Interface is up`,
			expected: true,
		},
		{
			name: "valid_is_down_output",
			input: `Port status:
Interface is down`,
			expected: true,
		},
		{
			name: "case_insensitive_validation",
			input: `INTERFACE STATUS
GIGABITETHERNET0/0/0 IS UP`,
			expected: true,
		},
		{
			name: "invalid_output_no_indicators",
			input: `This is some random output
without any interface indicators
just plain text`,
			expected: false,
		},
		{
			name:     "empty_output",
			input:    "",
			expected: false,
		},
		{
			name: "bgp_output_not_interface",
			input: `BGP router identifier 10.0.0.1
BGP table version is 1`,
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
		"show interfaces",
		"show interfaces status",
		"show vlans",
		"show interface brief",
	}

	assert.Equal(t, expectedCommands, commands)
	assert.Len(t, commands, 4)
}

// Test helper function parseStatus
func TestParseStatus(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"up", StatusUp},
		{"UP", StatusUp},
		{"Up", StatusUp},
		{"1", StatusUp},
		{"down", StatusDown},
		{"DOWN", StatusDown},
		{"Down", StatusDown},
		{"0", StatusDown},
		{"unknown", StatusDown},
		{"", StatusDown},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseStatus(tt.input)
			assert.Equal(t, tt.expected, result)
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
			input: `Gi0/0/0 is up, line protocol is up
!@#$%^&*() interface data
Hardware information available`,
		},
		{
			name: "unicode_characters",
			input: `Ethernet1/1 is up
αβγδε interface description
Hardware status available`,
		},
		{
			name: "malformed_interface_line",
			input: `This is not a valid interface line
GigabitEthernet0/0/0 status unknown format`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Should not panic and should return without error
			interfaces, err := parser.ParseInterfaces(tt.input)
			assert.NoError(t, err)
			assert.NotNil(t, interfaces)

			simpleInterfaces, err := parser.ParseSimpleInterfaces(tt.input)
			assert.NoError(t, err)
			assert.NotNil(t, simpleInterfaces)
		})
	}
}
