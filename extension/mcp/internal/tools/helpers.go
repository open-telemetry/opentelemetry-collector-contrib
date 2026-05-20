// Copyright The OpenTelemetry Authors
// Copyright 2025 Austin Parker
// SPDX-License-Identifier: Apache-2.0

package tools // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/mcp/internal/tools"

import (
	"fmt"

	"go.opentelemetry.io/collector/component"
)

// parseComponentKind validates and parses a component kind string into a component.Kind
func parseComponentKind(kindStr string) (component.Kind, error) {
	switch kindStr {
	case "receiver":
		return component.KindReceiver, nil
	case "processor":
		return component.KindProcessor, nil
	case "exporter":
		return component.KindExporter, nil
	case "connector":
		return component.KindConnector, nil
	case "extension":
		return component.KindExtension, nil
	default:
		return component.KindReceiver, fmt.Errorf("invalid component kind: %s (must be one of: receiver, processor, exporter, connector, extension)", kindStr)
	}
}

// validKindsMap returns a map of valid component kinds to their config section names
func validKindsMap() map[string]string {
	return map[string]string{
		"receiver":  "receivers",
		"processor": "processors",
		"exporter":  "exporters",
		"connector": "connectors",
		"extension": "extensions",
	}
}
