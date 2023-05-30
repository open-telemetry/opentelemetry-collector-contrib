// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azuremonitorexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azuremonitorexporter"

import (
	"time"

	"go.opentelemetry.io/collector/config/configopaque"
)

// Config defines configuration for Azure Monitor
type Config struct {
	Endpoint           string              `mapstructure:"endpoint"`
	InstrumentationKey configopaque.String `mapstructure:"instrumentation_key"`
	MaxBatchSize       int                 `mapstructure:"maxbatchsize"`
	MaxBatchInterval   time.Duration       `mapstructure:"maxbatchinterval"`
	SpanEventsEnabled  bool                `mapstructure:"spaneventsenabled"`
}
