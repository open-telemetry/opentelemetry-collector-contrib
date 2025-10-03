// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tinybirdexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/tinybirdexporter"

import "fmt"

// option represents a configuration option that can be passed to the tinybird exporter.
type option func(*tinybirdExporter) error

// withMaxRequestBodySize sets the maximum size of the request body in bytes.
func withMaxRequestBodySize(size int) option {
	return func(e *tinybirdExporter) error {
		if size <= 0 {
			return fmt.Errorf("max request body size must be positive, got %d", size)
		}
		e.maxRequestBodySize = size
		return nil
	}
}
