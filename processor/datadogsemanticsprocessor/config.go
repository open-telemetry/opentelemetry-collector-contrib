// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datadogsemanticsprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/datadogsemanticsprocessor"

// Config defines configuration for Datadog semantics processor.
type Config struct {
	// OverrideIncomingDatadogFields specifies what is done with incoming Datadog fields.
	// If it is false, any fields namespaced with "datadog." will pass through unchanged.
	// If it is true, all fields in the "datadog." namespace will be recomputed by the processor.
	// Default: false.
	OverrideIncomingDatadogFields bool `mapstructure:"override_incoming_datadog_fields"`
}
