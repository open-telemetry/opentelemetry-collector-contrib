// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package redfish // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/redfishreceiver/internal/redfish"

import "strings"

type ComputerSystem struct {
	ID           string `json:"Id"`
	AssetTag     string
	BiosVersion  string
	HostName     string
	Model        string
	Name         string
	Manufacturer string
	SerialNumber string
	SKU          string
	SystemType   string
	PowerState   string // On, Off, Unknown
	Status       Status
	Links        struct {
		Chassis []struct {
			Ref string `json:"@odata.id"`
		}
	}
}

// Redfish Generic Status
// State (Enabled, Disabled, Unknown)
// Health (Critical, OK, Warning)
type Status struct {
	State  string
	Health string
}

// Redfish Chassis
type Chassis struct {
	ID           string `json:"Id"`
	AssetTag     string
	ChassisType  string
	Manufacturer string
	Model        string
	Name         string
	SKU          string
	SerialNumber string
	PowerState   string // On, Off, Unknown
	Status       Status
	Thermal      struct {
		Ref string `json:"@odata.id"`
	}
}

type Fan struct {
	Name         string
	Reading      *int64
	ReadingUnits string
	Status       Status
}

type Temperature struct {
	Name           string
	ReadingCelsius *int64
	Status         Status
}

type Thermal struct {
	Fans         []Fan
	Temperatures []Temperature
}

// powerStateToMetric converts a redfish PowerState to a metric
func PowerStateToMetric(ps string) int64 {
	switch strings.ToLower(ps) {
	case "off":
		return 0
	case "on":
		return 1
	default:
		return -1
	}
}

// statusHealthToMetric converts a redfish Status.Health to a metric
func StatusHealthToMetric(sh string) int64 {
	switch strings.ToLower(sh) {
	case "critical":
		return 0
	case "ok":
		return 1
	case "warning":
		return 2
	default:
		return -1
	}
}

// statusStateToMetric converts a redfish Status.State to a metric
func StatusStateToMetric(ss string) int64 {
	switch strings.ToLower(ss) {
	case "disabled":
		return 0
	case "enabled":
		return 1
	default:
		return -1
	}
}
