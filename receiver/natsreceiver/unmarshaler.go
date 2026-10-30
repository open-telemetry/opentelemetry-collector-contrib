// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package natsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/natsreceiver"

import (
	"fmt"

	"go.opentelemetry.io/collector/component"
)

// newUnmarshaler returns the unmarshaler for a signal: the configured encoding
// extension if set, otherwise the built-in proto or json unmarshaler. U is the
// signal's pdata unmarshaler interface (e.g. plog.Unmarshaler); signal names it
// in errors.
func newUnmarshaler[U any](cfg *SignalConfig, host component.Host, signal string, proto, json U) (U, error) {
	var zero U
	if cfg.EncodingExtension != "" {
		var id component.ID
		if err := id.UnmarshalText([]byte(cfg.EncodingExtension)); err != nil {
			return zero, fmt.Errorf("failed to parse encoding extension name: %w", err)
		}
		ext, ok := host.GetExtensions()[id]
		if !ok {
			return zero, fmt.Errorf("encoding extension not found: %s", id)
		}
		u, ok := ext.(U)
		if !ok {
			return zero, fmt.Errorf("encoding extension %s does not support unmarshaling %s", id, signal)
		}
		return u, nil
	}

	switch cfg.Encoding {
	case "", encodingOTLPProto:
		return proto, nil
	case encodingOTLPJSON:
		return json, nil
	default:
		return zero, fmt.Errorf("unsupported encoding: %q", cfg.Encoding)
	}
}

// encodingName labels a signal's encoding in the receiver's obsreport metrics.
func encodingName(cfg *SignalConfig) string {
	switch {
	case cfg.EncodingExtension != "":
		return cfg.EncodingExtension
	case cfg.Encoding != "":
		return cfg.Encoding
	default:
		return encodingOTLPProto
	}
}
