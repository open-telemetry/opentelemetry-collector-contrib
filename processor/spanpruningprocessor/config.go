// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package spanpruningprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/spanpruningprocessor"

// Config holds the configuration for the SpanPruning processor.
type Config struct{}

func (*Config) Validate() error {
	return nil
}
