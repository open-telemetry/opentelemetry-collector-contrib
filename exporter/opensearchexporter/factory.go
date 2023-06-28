// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opensearchexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/opensearchexporter"

import (
	"go.opentelemetry.io/collector/exporter"
)

//  const (
//	// The value of "type" key in configuration.
//	typeStr = "opensearch"
//	// The stability level of the exporter.
//	stability = component.StabilityLevelDevelopment
//  )

// NewFactory creates a factory for OpenSearch exporter.
func NewFactory() exporter.Factory {
	//  return exporter.NewFactory(
	//	typeStr,
	//	createDefaultConfig,
	//	exporter.WithTraces(createTracesExporter, stability),
	//  )
	return nil
}
