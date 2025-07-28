// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package reader // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/parquet/reader"

// Config contains the parquet reader config options.
type Config struct {
	// If ProcessParallel is true, then functions which read multiple columns will read those columns in parallel
	// from the file with a number of readers equal to the number of columns. Otherwise columns are read serially.
	ProcessParallel bool `mapstructure:"process_parallel"`
	// BatchSize is the number of rows to read at a time from the file.
	BatchSize int `mapstructure:"batch_size" default:"1"`
}
