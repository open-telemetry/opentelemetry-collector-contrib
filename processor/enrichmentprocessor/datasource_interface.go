// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package enrichmentprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/enrichmentprocessor"

import (
	"context"
)

// DataSource represents an external data source for enrichment operations.
// It provides a clean interface for different data source implementations.
type DataSource interface {
	// Lookup performs a lookup for the given key and returns enrichment data
	Lookup(lookupField, key string) (enrichmentRow []string, index map[string]int, err error)

	// Start starts the data source (e.g., periodic refresh, connection establishment)
	Start(ctx context.Context) error

	// Stop stops the data source and cleans up resources
	Stop() error
}
