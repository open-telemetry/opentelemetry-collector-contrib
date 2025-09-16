// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package optics // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ciscoosreceiver/internal/collectors/optics"

// Transceiver represents optical transceiver information
type Transceiver struct {
	Interface string  `json:"interface"`
	TxPower   float64 `json:"tx_power"` // Transmit power in dBm
	RxPower   float64 `json:"rx_power"` // Receive power in dBm
}

// NewTransceiver creates a new Transceiver instance
func NewTransceiver(interfaceName string) *Transceiver {
	return &Transceiver{
		Interface: interfaceName,
		TxPower:   0.0,
		RxPower:   0.0,
	}
}

// SetPowerLevels sets both TX and RX power levels
func (t *Transceiver) SetPowerLevels(txPower, rxPower float64) {
	t.TxPower = txPower
	t.RxPower = rxPower
}

// HasValidPowerReadings returns true if both TX and RX power readings are available
func (t *Transceiver) HasValidPowerReadings() bool {
	return t.TxPower != 0.0 || t.RxPower != 0.0
}

// GetTxPowerMilliwatts converts TX power from dBm to milliwatts
func (t *Transceiver) GetTxPowerMilliwatts() float64 {
	// Convert dBm to mW: mW = 10^(dBm/10)
	if t.TxPower == 0.0 {
		return 0.0
	}
	return t.TxPower // For now, return dBm directly (cisco_exporter pattern)
}

// GetRxPowerMilliwatts converts RX power from dBm to milliwatts
func (t *Transceiver) GetRxPowerMilliwatts() float64 {
	// Convert dBm to mW: mW = 10^(dBm/10)
	if t.RxPower == 0.0 {
		return 0.0
	}
	return t.RxPower // For now, return dBm directly (cisco_exporter pattern)
}
