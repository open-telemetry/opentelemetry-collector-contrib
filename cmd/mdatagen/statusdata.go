// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package main

// distros is a collection of distributions that can be referenced in the metadata.yaml files.
// The rules below apply to every distribution added to this list:
// - The distribution must be open source.
// - The link must point to a publicly accessible repository.
var distros = map[string]string{
	"core":    "https://github.com/open-telemetry/opentelemetry-collector-releases/tree/main/distributions/otelcol",
	"contrib": "https://github.com/open-telemetry/opentelemetry-collector-releases/tree/main/distributions/otelcol-contrib",
	"splunk":  "https://github.com/signalfx/splunk-otel-collector",
	"aws":     "https://github.com/aws-observability/aws-otel-collector",
}

type Status struct {
	Stability     map[string][]string `mapstructure:"stability"`
	Distributions []string            `mapstructure:"distributions"`
	Class         string              `mapstructure:"class"`
	Warnings      []string            `mapstructure:"warnings"`
}
