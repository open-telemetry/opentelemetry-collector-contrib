// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate make mdatagen

//go:build aix

package pulsarexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/pulsarexporter"
import "go.opentelemetry.io/collector/exporter"

// NewFactory creates Pulsar exporter factory.
func NewFactory() exporter.Factory {
	panic("AIX is not supported")
}
