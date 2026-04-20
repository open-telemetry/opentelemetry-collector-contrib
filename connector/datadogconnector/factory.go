// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate make mdatagen

//go:build !aix

package datadogconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/datadogconnector"

import (
	"go.opentelemetry.io/collector/connector"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/datadogconnector/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog/apmstats"
)

// NewFactory creates a factory for datadog connector.
func NewFactory() connector.Factory {
	//  OTel connector factory to make a factory for connectors
	return apmstats.NewConnectorFactory(metadata.Type, metadata.TracesToMetricsStability, metadata.TracesToTracesStability, nil, nil, nil)
}
