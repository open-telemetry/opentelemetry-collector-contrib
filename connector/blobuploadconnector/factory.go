// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package blobuploadconnector

import (
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/connector"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/blobuploadconnector/internal/metadata"
)

func createDefaultConfig() component.Config {
	return NewDefaultConfig()
}

// NewFactoryWithDeps instantiates the factory with dependency injection, allowing
// for a more testable interface that allows global functions/objects to be swapped out.
func NewFactoryWithDeps(deps Deps) connector.Factory {
	return connector.NewFactory(
		metadata.Type,
		createDefaultConfig,
		connector.WithTracesToTraces(
			createTracesToTracesConnector(deps),
			metadata.TracesToTracesStability),
		connector.WithLogsToLogs(
			createLogsToLogsConnector(deps),
			metadata.LogsToLogsStability))
}

// NewFactory is used by the OTel collector to instantiate this component.
func NewFactory() connector.Factory {
	return NewFactoryWithDeps(NewDeps())
}
