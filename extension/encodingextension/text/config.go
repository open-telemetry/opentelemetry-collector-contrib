// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package text // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encodingextension/text"

type Config struct {
	encoding string `mapstructure:"encoding"`
}
