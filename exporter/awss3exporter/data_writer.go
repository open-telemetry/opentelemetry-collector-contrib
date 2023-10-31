// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awss3exporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awss3exporter"

import "context"

type dataWriter interface {
	writeBuffer(ctx context.Context, buf []byte, config *Config, metadata string, format string) error
}
