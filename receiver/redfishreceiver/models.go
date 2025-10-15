// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package redfishreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/redfishreceiver"

import "strings"

type computerSystem struct {
	Id           string
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
	Status       status
	Links        struct {
		Chassis []struct {
			Ref string `json:"@odata.id"`
		}
	}
}

// Redfish Generic Status
// State (Enabled, Disabled, Unknown)
// Health (Critical, OK, Warning)
type status struct {
	State  string
	Health string
}

// Redfish Chassis
type chassis struct {
	Id           string
	AssetTag     string
	ChassisType  string
	Manufacturer string
	Model        string
	Name         string
	SKU          string
	SerialNumber string
	PowerState   string // On, Off, Unknown
	Status       status
	Thermal      struct {
		Ref string `json:"@odata.id"`
	}
}

type fan struct {
	Name         string
	Reading      *int64
	ReadingUnits *string
	Status       status
}

type temperature struct {
	Name           string
	ReadingCelsius *float64
	Status         status
}

type thermal struct {
	Fans         []fan
	Temperatures []temperature
}

// powerStateToMetric converts a redfish PowerState to a metric
func powerStateToMetric(ps string) int64 {
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
func statusHealthToMetric(sh string) int64 {
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
func statusStateToMetric(ss string) int64 {
	switch strings.ToLower(ss) {
	case "disabled":
		return 0
	case "enabled":
		return 1
	default:
		return -1
	}
}
