// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package redfishreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/redfishreceiver"

type Resource string

const (
	ComputerSystemResource Resource = "ComputerSystem"
	ChassisResource        Resource = "Chassis"
	FansResource           Resource = "Fans"
	TemperaturesResource   Resource = "Temperatures"
)
