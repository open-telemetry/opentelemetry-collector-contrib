// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azurefunctionsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azurefunctionsreceiver"

import (
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

// loadEncodingExtension loads an extension by ID from the host
// Returns an error if the extension is missing or does not implement the expected type.
func loadEncodingExtension[T any](host component.Host, id component.ID, signalType string) (T, error) {
	var zero T
	ext, ok := host.GetExtensions()[id]
	if !ok {
		return zero, fmt.Errorf("extension %q not found", id.String())
	}
	u, ok := ext.(T)
	if !ok {
		return zero, fmt.Errorf("extension %q is not a %s unmarshaler", id.String(), signalType)
	}
	return u, nil
}

// loadLogsUnmarshalers builds a map of binding name to plog.Unmarshaler by loading
// each encoding extension from the host. The extension component must implement plog.Unmarshaler.
func loadLogsUnmarshalers(host component.Host, bindings []EncodingConfig) (map[string]plog.Unmarshaler, error) {
	out := make(map[string]plog.Unmarshaler, len(bindings))
	for _, b := range bindings {
		u, err := loadEncodingExtension[plog.Unmarshaler](host, b.Encoding, "logs")
		if err != nil {
			return nil, fmt.Errorf("binding %q: %w", b.Name, err)
		}
		out[b.Name] = u
	}
	return out, nil
}

// loadMetricsUnmarshalers builds a map of binding name to pmetric.Unmarshaler by loading
// each encoding extension from the host. The extension component must implement pmetric.Unmarshaler.
func loadMetricsUnmarshalers(host component.Host, bindings []EncodingConfig) (map[string]pmetric.Unmarshaler, error) {
	out := make(map[string]pmetric.Unmarshaler, len(bindings))
	for _, b := range bindings {
		u, err := loadEncodingExtension[pmetric.Unmarshaler](host, b.Encoding, "metrics")
		if err != nil {
			return nil, fmt.Errorf("binding %q: %w", b.Name, err)
		}
		out[b.Name] = u
	}
	return out, nil
}
