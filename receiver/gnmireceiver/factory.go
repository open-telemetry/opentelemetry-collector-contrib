// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package gnmireceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/gnmireceiver"

import (
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/receiver"
)

// NewFactory creates a receiver factory for the gNMI receiver.
func NewFactory() receiver.Factory {
	return receiver.NewFactory(component.MustNewType("gnmi"), createDefaultConfig)
}

func createDefaultConfig() component.Config {
	return &Config{}
}
