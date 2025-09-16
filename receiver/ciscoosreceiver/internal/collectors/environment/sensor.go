// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package environment // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ciscoosreceiver/internal/collectors/environment"

import (
	"fmt"
	"strings"
)

// SensorType represents the type of environmental sensor
type SensorType string

const (
	TemperatureSensor SensorType = "temperature"
	PowerSupplySensor SensorType = "power_supply"
	FanSensor         SensorType = "fan"
	VoltageSensor     SensorType = "voltage"
)

// Sensor represents an environmental sensor reading
type Sensor struct {
	Name      string
	Type      SensorType
	Value     float64
	Unit      string
	Status    string
	Location  string
	IsHealthy bool
	Threshold float64
}

// NewTemperatureSensor creates a new temperature sensor
func NewTemperatureSensor(name, location string, temperature float64) *Sensor {
	return &Sensor{
		Name:      name,
		Type:      TemperatureSensor,
		Value:     temperature,
		Unit:      "celsius",
		Location:  location,
		IsHealthy: temperature < 80.0, // Default threshold
		Threshold: 80.0,
		Status:    "OK",
	}
}

// NewPowerSupplySensor creates a new power supply sensor
func NewPowerSupplySensor(name, status string) *Sensor {
	isHealthy := status == "OK" || status == "Normal"

	return &Sensor{
		Name:      name,
		Type:      PowerSupplySensor,
		Value:     0, // Power supplies typically report status, not numeric value
		Unit:      "status",
		Status:    status,
		IsHealthy: isHealthy,
	}
}

// NewFanSensor creates a new fan sensor
func NewFanSensor(name, location string, rpm float64) *Sensor {
	return &Sensor{
		Name:      name,
		Type:      FanSensor,
		Value:     rpm,
		Unit:      "rpm",
		Location:  location,
		IsHealthy: rpm > 0, // Fan is healthy if spinning
		Status:    "OK",
	}
}

// SetStatus updates the sensor status and health
func (s *Sensor) SetStatus(status string) {
	s.Status = status

	// Determine health based on status (case-insensitive matching)
	healthyStatuses := []string{"OK", "Normal", "Good", "Operational", "ok", "normal", "good", "operational"}
	s.IsHealthy = false

	for _, healthyStatus := range healthyStatuses {
		if strings.EqualFold(s.Status, healthyStatus) {
			s.IsHealthy = true
			break
		}
	}
}

// GetHealthStatus returns 1 if sensor is healthy, 0 if not
func (s *Sensor) GetHealthStatus() int64 {
	if s.IsHealthy {
		return 1
	}
	return 0
}

// GetNumericValue returns the sensor value as int64 for metrics
func (s *Sensor) GetNumericValue() int64 {
	return int64(s.Value)
}

// String returns a string representation of the sensor
func (s *Sensor) String() string {
	return fmt.Sprintf("%s Sensor %s: %.2f %s (%s) - %s",
		s.Type, s.Name, s.Value, s.Unit, s.Location, s.Status)
}

// Validate checks if the sensor has valid data
func (s *Sensor) Validate() bool {
	return s.Name != "" && s.Type != ""
}

// IsTemperature checks if this is a temperature sensor
func (s *Sensor) IsTemperature() bool {
	return s.Type == TemperatureSensor
}

// IsPowerSupply checks if this is a power supply sensor
func (s *Sensor) IsPowerSupply() bool {
	return s.Type == PowerSupplySensor
}

// IsFan checks if this is a fan sensor
func (s *Sensor) IsFan() bool {
	return s.Type == FanSensor
}
