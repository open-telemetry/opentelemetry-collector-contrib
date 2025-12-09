// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureencodingextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/azureencodingextension"

import (
	"fmt"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/azureencodingextension/internal/unmarshaler"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/azureencodingextension/internal/unmarshaler/logs"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/azureencodingextension/internal/unmarshaler/metrics"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/azureencodingextension/internal/unmarshaler/traces"
)

type Config struct {
	Traces  traces.TracesConfig   `mapstructure:"traces"`
	Logs    logs.LogsConfig       `mapstructure:"logs"`
	Metrics metrics.MetricsConfig `mapstructure:"metrics"`

	// Format of exported records, can be either "eventhub" or "blobstorage"
	Format unmarshaler.RecordsBatchFormat `mapstructure:"format"`

	// prevent unkeyed literal initialization
	_ struct{}
}

func (c *Config) Validate() error {
	if c.Format != unmarshaler.FormatEventHub && c.Format != unmarshaler.FormatBlobStorage {
		return fmt.Errorf("unsupported format: %q", c.Format)
	}

	return nil
}
