// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package helper // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"

import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/decoder"

// Deprecated: [v0.84.0]
func NewEncodingConfig() EncodingConfig {
	return EncodingConfig{
		Encoding: "utf-8",
	}
}

// Deprecated: [v0.84.0]
type EncodingConfig struct {
	Encoding string `mapstructure:"encoding,omitempty"`
}

// Deprecated: [v0.84.0] Use decoder.Decoder instead
type Decoder = decoder.Decoder

var NewDecoder = decoder.New

// Deprecated: [v0.84.0] Use decoder.LookupEncoding instead
var LookupEncoding = decoder.LookupEncoding

// Deprecated: [v0.84.0] Use decoder.IsNop instead
var IsNop = decoder.IsNop
