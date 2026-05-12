// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build aix

package pulsarreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/pulsarreceiver"
import (
	"go.opentelemetry.io/collector/receiver"
)

// NewFactory creates Pulsar exporter factory.
func NewFactory() receiver.Factory {
	panic("AIX is not supported")
}
