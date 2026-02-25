// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package interfacesscraper

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestParseInterfaces_IOSXE(t *testing.T) {
	output := `
GigabitEthernet0/0 is up, line protocol is up
  Hardware is iGbE, address is aabb.ccdd.ee01 (bia aabb.ccdd.ee01)
  Description: Uplink to Core
  Internet address is 10.1.1.1/24
  MTU 1500 bytes, BW 1000000 Kbit/sec, DLY 10 usec,
     reliability 255/255, txload 1/255, rxload 1/255
  Encapsulation ARPA, loopback not set
  Keepalive set (10 sec)
  Full-duplex, 1000 Mb/s, media type is RJ45
  output flow-control is unsupported, input flow-control is unsupported
  ARP type: ARPA, ARP Timeout 04:00:00
  Last input 00:00:00, output 00:00:00, output hang never
  Last clearing of "show interface" counters never
  Input queue: 0/75/0/0 (size/max/drops/flushes); Total output drops: 5
  Queueing strategy: fifo
  Output queue: 0/40 (size/max)
  5 minute input rate 1000 bits/sec, 2 packets/sec
  5 minute output rate 2000 bits/sec, 3 packets/sec
     12345 packets input, 9876543 bytes, 0 no buffer
     Received 150 broadcasts (75 IP multicasts)
     0 runts, 0 giants, 0 throttles
     10 input errors, 0 CRC, 0 frame, 0 overrun, 0 ignored
     0 watchdog, 25 multicast, 0 pause input
     20 packets output, 1234567 bytes, 0 underruns
     5 output errors, 0 collisions, 1 interface resets
     0 unknown protocol drops
     0 babbles, 0 late collision, 0 deferred
     0 lost carrier, 0 no carrier, 0 pause output
     0 output buffer failures, 0 output buffers swapped out

TenGigabitEthernet1/0/1 is down, line protocol is down (notconnect)
  Hardware is Ten Gigabit Ethernet, address is 1122.3344.5566 (bia 1122.3344.5566)
  MTU 1500 bytes, BW 10000000 Kbit/sec, DLY 10 usec,
  Encapsulation ARPA, loopback not set
  Input queue: 0/75/2/0 (size/max/drops/flushes); Total output drops: 10
  100 packets input, 5000 bytes
  5 input errors, 0 CRC, 0 frame, 0 overrun, 0 ignored
  50 packets output, 3000 bytes
  3 output errors, 0 collisions, 0 interface resets
`

	logger := zaptest.NewLogger(t)
	interfaces := parseInterfaces(output, logger)

	require.Len(t, interfaces, 2, "should parse 2 interfaces")

	// Verify GigabitEthernet0/0
	gig0 := interfaces[0]
	assert.Equal(t, "GigabitEthernet0/0", gig0.Name)
	assert.Equal(t, "aabb.ccdd.ee01", gig0.MACAddress)
	assert.Equal(t, "Uplink to Core", gig0.Description)
	assert.Equal(t, StatusUp, gig0.OperStatus)
	assert.Equal(t, float64(9876543), gig0.InputBytes)
	assert.Equal(t, float64(1234567), gig0.OutputBytes)
	assert.Equal(t, float64(10), gig0.InputErrors)
	assert.Equal(t, float64(5), gig0.OutputErrors)
	assert.Equal(t, float64(0), gig0.InputDrops)
	assert.Equal(t, float64(5), gig0.OutputDrops)
	assert.Equal(t, float64(150), gig0.InputBroadcast)
	assert.Equal(t, float64(75), gig0.InputMulticast)
	assert.Equal(t, "1000 Mb/s", gig0.SpeedString)

	// Verify TenGigabitEthernet1/0/1
	ten1 := interfaces[1]
	assert.Equal(t, "TenGigabitEthernet1/0/1", ten1.Name)
	assert.Equal(t, "1122.3344.5566", ten1.MACAddress)
	assert.Equal(t, StatusDown, ten1.OperStatus)
	assert.Equal(t, float64(5000), ten1.InputBytes)
	assert.Equal(t, float64(3000), ten1.OutputBytes)
	assert.Equal(t, float64(5), ten1.InputErrors)
	assert.Equal(t, float64(3), ten1.OutputErrors)
	assert.Equal(t, float64(2), ten1.InputDrops)
	assert.Equal(t, float64(10), ten1.OutputDrops)
}

func TestParseInterfaces_NXOS(t *testing.T) {
	output := `
Ethernet1/1 is up
admin state is up, Dedicated Interface
  Hardware: 1000/10000 Ethernet, address: 2233.4455.6677 (bia 2233.4455.6677)
  Description: Server Connection
  MTU 1500 bytes, BW 10000000 Kbit, DLY 10 usec
  reliability 255/255, txload 1/255, rxload 1/255
  Encapsulation ARPA, medium is broadcast
  full-duplex, 10 Gb/s, media type is 10G
  Beacon is turned off
  Auto-Negotiation is turned on
  Input flow-control is off, output flow-control is off
  Auto-mdix is turned off
  Switchport monitor is off
  EtherType is 0x8100
  Last link flapped 5week(s) 2day(s)
  Last clearing of "show interface" counters never
  1 interface resets
  30 seconds input rate 16 bits/sec, 0 packets/sec
  30 seconds output rate 24 bits/sec, 0 packets/sec
  Load-Interval #2: 5 minute (300 seconds)
    input rate 8 bps, 0 pps; output rate 16 bps, 0 pps
  RX
    54321 unicast packets  9999 multicast packets  7777 broadcast packets
    72097 input packets  987654321 bytes
    0 jumbo packets  0 storm suppression bytes
    0 runts  0 giants  0 CRC  0 no buffer
    0 input error  0 short frame  0 overrun   0 underrun  0 ignored
    0 watchdog  0 bad etype drop  0 bad proto drop  0 if down drop
    0 input with dribble  0 input discard
    25 Rx pause
  TX
    12345 unicast packets  5555 multicast packets  3333 broadcast packets
    21233 output packets  123456789 bytes
    0 jumbo packets
    0 output error  0 collision  0 deferred  0 late collision
    0 lost carrier  0 no carrier  0 babble  0 output discard
    0 Tx pause

mgmt0 is up
admin state is up,
  Hardware: GigabitEthernet, address: aabb.ccdd.eeff (bia aabb.ccdd.eeff)
  Internet Address is 192.168.1.10/24
  MTU 1500 bytes, BW 1000000 Kbit, DLY 10 usec
  RX
    1000 unicast packets  500 multicast packets  200 broadcast packets
    1700 input packets  850000 bytes
  TX
    800 unicast packets  100 multicast packets  50 broadcast packets
    950 output packets  475000 bytes
`

	logger := zaptest.NewLogger(t)
	interfaces := parseInterfaces(output, logger)

	require.Len(t, interfaces, 2, "should parse 2 interfaces")

	// Verify Ethernet1/1
	eth1 := interfaces[0]
	assert.Equal(t, "Ethernet1/1", eth1.Name)
	assert.Equal(t, "2233.4455.6677", eth1.MACAddress)
	assert.Equal(t, "Server Connection", eth1.Description)
	assert.Equal(t, StatusUp, eth1.OperStatus)
	assert.Equal(t, float64(987654321), eth1.InputBytes)
	assert.Equal(t, float64(123456789), eth1.OutputBytes)
	assert.Equal(t, float64(9999), eth1.InputMulticast)
	assert.Equal(t, float64(7777), eth1.InputBroadcast)

	// Verify mgmt0
	mgmt := interfaces[1]
	assert.Equal(t, "mgmt0", mgmt.Name)
	assert.Equal(t, "aabb.ccdd.eeff", mgmt.MACAddress)
	assert.Equal(t, StatusUp, mgmt.OperStatus)
	assert.Equal(t, float64(850000), mgmt.InputBytes)
	assert.Equal(t, float64(475000), mgmt.OutputBytes)
	assert.Equal(t, float64(500), mgmt.InputMulticast)
	assert.Equal(t, float64(200), mgmt.InputBroadcast)
}

func TestParseInterfaces_VirtualInterfaces(t *testing.T) {
	output := `
Loopback0 is up, line protocol is up
  Hardware is Loopback
  Internet address is 1.1.1.1/32
  MTU 1514 bytes, BW 8000000 Kbit/sec, DLY 5000 usec,
  Input queue: 0/75/0/0 (size/max/drops/flushes); Total output drops: 0
  100 packets input, 10000 bytes
  0 input errors, 0 CRC, 0 frame, 0 overrun, 0 ignored
  100 packets output, 10000 bytes
  0 output errors, 0 collisions, 0 interface resets

Vlan100 is up, line protocol is up
  Hardware is Ethernet SVI, address is 0011.2233.4455 (bia 0011.2233.4455)
  Description: Management VLAN
  Internet address is 10.0.100.1/24
  MTU 1500 bytes, BW 1000000 Kbit/sec, DLY 10 usec,
  Input queue: 0/75/1/0 (size/max/drops/flushes); Total output drops: 2
  500 packets input, 50000 bytes
  Received 30 broadcasts (15 IP multicasts)
  2 input errors, 0 CRC, 0 frame
  400 packets output, 40000 bytes
  1 output errors, 0 collisions, 0 interface resets
`

	logger := zaptest.NewLogger(t)
	interfaces := parseInterfaces(output, logger)

	require.Len(t, interfaces, 2, "should parse 2 interfaces")

	// Verify Loopback0 (no MAC address)
	loopback := interfaces[0]
	assert.Equal(t, "Loopback0", loopback.Name)
	assert.Empty(t, loopback.MACAddress, "loopback should have no MAC")
	assert.Equal(t, StatusUp, loopback.OperStatus)
	assert.Equal(t, float64(10000), loopback.InputBytes)
	assert.Equal(t, float64(10000), loopback.OutputBytes)

	// Verify Vlan100
	vlan := interfaces[1]
	assert.Equal(t, "Vlan100", vlan.Name)
	assert.Equal(t, "0011.2233.4455", vlan.MACAddress)
	assert.Equal(t, "Management VLAN", vlan.Description)
	assert.Equal(t, StatusUp, vlan.OperStatus)
	assert.Equal(t, float64(50000), vlan.InputBytes)
	assert.Equal(t, float64(40000), vlan.OutputBytes)
	assert.Equal(t, float64(2), vlan.InputErrors)
	assert.Equal(t, float64(1), vlan.OutputErrors)
	assert.Equal(t, float64(1), vlan.InputDrops)
	assert.Equal(t, float64(2), vlan.OutputDrops)
	assert.Equal(t, float64(30), vlan.InputBroadcast)
	assert.Equal(t, float64(15), vlan.InputMulticast)
}

func TestParseInterfaces_AdminDown(t *testing.T) {
	output := `
GigabitEthernet0/1 is administratively down, line protocol is down
  Hardware is iGbE, address is 1111.2222.3333 (bia 1111.2222.3333)
  MTU 1500 bytes, BW 1000000 Kbit/sec
  Input queue: 0/75/0/0 (size/max/drops/flushes); Total output drops: 0
  0 packets input, 0 bytes
  0 input errors, 0 CRC, 0 frame, 0 overrun, 0 ignored
  0 packets output, 0 bytes
  0 output errors, 0 collisions, 0 interface resets
`

	logger := zaptest.NewLogger(t)
	interfaces := parseInterfaces(output, logger)

	require.Len(t, interfaces, 1, "should parse 1 interface")

	iface := interfaces[0]
	assert.Equal(t, "GigabitEthernet0/1", iface.Name)
	assert.Equal(t, "1111.2222.3333", iface.MACAddress)
	assert.Equal(t, StatusDown, iface.OperStatus)
	assert.Equal(t, float64(0), iface.InputBytes)
	assert.Equal(t, float64(0), iface.OutputBytes)
}

func TestParseSimpleInterfaces(t *testing.T) {
	output := `
Interface              IP-Address      OK? Method Status                Protocol
GigabitEthernet0/0     10.1.1.1        YES NVRAM  up                    up
GigabitEthernet0/1     unassigned      YES NVRAM  administratively down down
TenGigabitEthernet1/1  10.2.2.1        YES NVRAM  up                    up
Loopback0              1.1.1.1         YES NVRAM  up                    up
`

	logger := zaptest.NewLogger(t)
	interfaces := parseSimpleInterfaces(output, logger)

	require.Len(t, interfaces, 4, "should parse 4 interfaces")

	assert.Equal(t, "GigabitEthernet0/0", interfaces[0].Name)
	assert.Equal(t, StatusUp, interfaces[0].OperStatus)

	assert.Equal(t, "GigabitEthernet0/1", interfaces[1].Name)
	assert.Equal(t, StatusDown, interfaces[1].OperStatus)

	assert.Equal(t, "TenGigabitEthernet1/1", interfaces[2].Name)
	assert.Equal(t, StatusUp, interfaces[2].OperStatus)

	assert.Equal(t, "Loopback0", interfaces[3].Name)
	assert.Equal(t, StatusUp, interfaces[3].OperStatus)
}

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

func TestFormatSpeed(t *testing.T) {
	tests := []struct {
		name     string
		input    int64
		expected string
	}{
		{"zero", 0, ""},
		{"negative", -100, ""},
		{"1 Kbps", 1000, "1 Kb/s"},
		{"100 Kbps", 100000, "100 Kb/s"},
		{"1 Mbps", 1000000, "1 Mb/s"},
		{"100 Mbps", 100000000, "100 Mb/s"},
		{"1 Gbps", 1000000000, "1 Gb/s"},
		{"10 Gbps", 10000000000, "10 Gb/s"},
		{"100 Gbps", 100000000000, "100 Gb/s"},
		{"500 bps", 500, "500 b/s"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatSpeed(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestStr2Float64(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected float64
	}{
		{"valid integer", "123", 123.0},
		{"valid float", "123.45", 123.45},
		{"dash", "-", 0.0},
		{"empty", "", 0.0},
		{"invalid", "abc", 0.0},
		{"zero", "0", 0.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := str2float64(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestInterface_GetOperStatusInt(t *testing.T) {
	tests := []struct {
		name     string
		status   string
		expected int64
	}{
		{"up", StatusUp, 1},
		{"down", StatusDown, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			iface := &Interface{OperStatus: tt.status}
			result := iface.GetOperStatusInt()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestInterface_Validate(t *testing.T) {
	tests := []struct {
		name     string
		iface    *Interface
		expected bool
	}{
		{
			name:     "valid interface",
			iface:    &Interface{Name: "eth0", OperStatus: StatusUp},
			expected: true,
		},
		{
			name:     "empty name",
			iface:    &Interface{Name: "", OperStatus: StatusUp},
			expected: false,
		},
		{
			name:     "invalid status gets normalized",
			iface:    &Interface{Name: "eth0", OperStatus: "invalid"},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.iface.Validate()
			assert.Equal(t, tt.expected, result)
			if tt.name == "invalid status gets normalized" {
				assert.Equal(t, StatusDown, tt.iface.OperStatus)
			}
		})
	}
}
