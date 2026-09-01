// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azurefunctionsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azurefunctionsreceiver"

import (
	"fmt"

	"go.opentelemetry.io/collector/component"
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

// loadUnmarshalers builds a map of binding name to unmarshaler by loading each
// encoding extension from the host. The extension component must implement T.
func loadUnmarshalers[T any](host component.Host, bindings []EncodingConfig, signalType string) (map[string]T, error) {
	out := make(map[string]T, len(bindings))
	for _, b := range bindings {
		u, err := loadEncodingExtension[T](host, b.Encoding, signalType)
		if err != nil {
			return nil, fmt.Errorf("binding %q: %w", b.Name, err)
		}
		out[b.Name] = u
	}
	return out, nil
}
