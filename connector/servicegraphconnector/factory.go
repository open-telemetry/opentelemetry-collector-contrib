// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate mdatagen metadata.yaml

package servicegraphconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/servicegraphconnector"

import (
	"go.opentelemetry.io/collector/connector"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/servicegraphconnector/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/servicegraphprocessor"
)

// NewFactory returns a ConnectorFactory.
func NewFactory() connector.Factory {
	return servicegraphprocessor.NewConnectorFactoryFunc(metadata.Type, metadata.TracesToMetricsStability)
}
