// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package jaeger // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encodingextension/jaeger"

type Config struct {
	Protocol string `mapstructure:"protocol"`
}
