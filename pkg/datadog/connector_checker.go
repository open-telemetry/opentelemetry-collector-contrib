// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datadog // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog"

import "go.opentelemetry.io/collector/component"

// ConnectorChecker is an interface that can be implemented by extensions to
// allow other components (e.g., exporters) to query whether a specific
// connector type is configured in the collector pipeline.
//
// Components can retrieve this interface at startup via host.GetExtensions()
// and type-assert to ConnectorChecker.
type ConnectorChecker interface {
	// HasConnector reports whether at least one connector of the given type
	// is configured in the collector pipeline.
	HasConnector(connectorType component.Type) bool
}
