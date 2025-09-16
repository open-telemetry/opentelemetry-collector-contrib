// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package interfaces // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ciscoosreceiver/internal/collectors/interfaces"

import "fmt"

// Status constants for cisco_exporter compatibility
const (
	StatusUp   = "up"
	StatusDown = "down"
)

// Interface represents a network interface on a Cisco device - cisco_exporter compatible
type Interface struct {
	Name        string
	MACAddress  string
	Description string

	AdminStatus string
	OperStatus  string

	InputErrors  float64
	OutputErrors float64

	InputDrops  float64
	OutputDrops float64

	InputBytes  float64
	OutputBytes float64

	InputPkts  float64
	OutputPkts float64

	InputBroadcast float64
	InputMulticast float64

	Speed       int64
	SpeedString string
	MTU         int64
	VLANs       []string
}

// NewInterface creates a new Interface with default values
func NewInterface(name string) *Interface {
	return &Interface{
		Name:        name,
		AdminStatus: StatusDown,
		OperStatus:  StatusDown,
		VLANs:       make([]string, 0),
	}
}

// SetStatus sets both administrative and operational status
func (i *Interface) SetStatus(adminStatus, operStatus string) {
	i.AdminStatus = parseStatus(adminStatus)
	i.OperStatus = parseStatus(operStatus)
}

// SetAdminStatus sets the administrative status
func (i *Interface) SetAdminStatus(status string) {
	i.AdminStatus = parseStatus(status)
}

// SetOperStatus sets the operational status
func (i *Interface) SetOperStatus(status string) {
	i.OperStatus = parseStatus(status)
}

// parseStatus converts string status to status string
func parseStatus(status string) string {
	switch status {
	case "up", "UP", "Up", "1":
		return StatusUp
	case "down", "DOWN", "Down", "0":
		return StatusDown
	default:
		return StatusDown
	}
}

// GetAdminStatusInt returns admin status as integer (1=up, 0=down)
func (i *Interface) GetAdminStatusInt() int64 {
	if i.AdminStatus == StatusUp {
		return 1
	}
	return 0
}

// GetOperStatusInt returns operational status as integer (1=up, 0=down)
func (i *Interface) GetOperStatusInt() int64 {
	if i.OperStatus == StatusUp {
		return 1
	}
	return 0
}

// SetTrafficCounters sets traffic counter values
func (i *Interface) SetTrafficCounters(inBytes, outBytes, inPkts, outPkts float64) {
	i.InputBytes = inBytes
	i.OutputBytes = outBytes
	i.InputPkts = inPkts
	i.OutputPkts = outPkts
}

// SetErrorCounters sets error counter values
func (i *Interface) SetErrorCounters(inErrors, outErrors float64) {
	i.InputErrors = inErrors
	i.OutputErrors = outErrors
}

// SetDropCounters sets drop counter values
func (i *Interface) SetDropCounters(inDrops, outDrops float64) {
	i.InputDrops = inDrops
	i.OutputDrops = outDrops
}

// SetBroadcastMulticastCounters sets broadcast and multicast counter values
func (i *Interface) SetBroadcastMulticastCounters(broadcast, multicast float64) {
	i.InputBroadcast = broadcast
	i.InputMulticast = multicast
}

// SetMACAddress sets the MAC address
func (i *Interface) SetMACAddress(mac string) {
	i.MACAddress = mac
}

// HasErrorStatus checks if admin and operational status differ (cisco_exporter compatibility)
func (i *Interface) HasErrorStatus() bool {
	return i.AdminStatus != i.OperStatus
}

// GetErrorStatusInt returns error status as integer (1=error, 0=no error)
func (i *Interface) GetErrorStatusInt() int64 {
	if i.HasErrorStatus() {
		return 1
	}
	return 0
}

// AddVLAN adds a VLAN to the interface
func (i *Interface) AddVLAN(vlan string) {
	i.VLANs = append(i.VLANs, vlan)
}

// SetVLANs sets the VLAN list for the interface
func (i *Interface) SetVLANs(vlans []string) {
	i.VLANs = vlans
}

// IsUp returns true if both admin and operational status are up
func (i *Interface) IsUp() bool {
	return i.AdminStatus == StatusUp && i.OperStatus == StatusUp
}

// IsAdminUp returns true if administrative status is up
func (i *Interface) IsAdminUp() bool {
	return i.AdminStatus == StatusUp
}

// IsOperUp returns true if operational status is up
func (i *Interface) IsOperUp() bool {
	return i.OperStatus == StatusUp
}

// HasErrors returns true if the interface has input or output errors
func (i *Interface) HasErrors() bool {
	return i.InputErrors > 0 || i.OutputErrors > 0
}

// GetTotalBytes returns total bytes (input + output)
func (i *Interface) GetTotalBytes() float64 {
	return i.InputBytes + i.OutputBytes
}

// GetTotalPackets returns total packets (input + output)
func (i *Interface) GetTotalPackets() float64 {
	return i.InputPkts + i.OutputPkts
}

// GetTotalErrors returns total errors (input + output)
func (i *Interface) GetTotalErrors() float64 {
	return i.InputErrors + i.OutputErrors
}

// String returns a string representation of the interface
func (i *Interface) String() string {
	return fmt.Sprintf("Interface %s: Admin=%s, Oper=%s, Speed=%d, MTU=%d, In=%.0f bytes, Out=%.0f bytes",
		i.Name, i.AdminStatus, i.OperStatus, i.Speed, i.MTU, i.InputBytes, i.OutputBytes)
}

// Validate checks if the interface has valid data
func (i *Interface) Validate() bool {
	return i.Name != ""
}

// IsPhysical checks if this is a physical interface (not loopback, VLAN, etc.)
func (i *Interface) IsPhysical() bool {
	// Simple heuristic - physical interfaces typically contain numbers
	// and don't start with common virtual interface prefixes
	name := i.Name
	if len(name) == 0 {
		return false
	}

	// Common virtual interface prefixes
	virtualPrefixes := []string{"Loopback", "Vlan", "Tunnel", "Port-channel"}

	for _, prefix := range virtualPrefixes {
		if len(name) >= len(prefix) && name[:len(prefix)] == prefix {
			return false
		}
	}

	return true
}
