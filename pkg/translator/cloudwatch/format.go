// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cloudwatch

import "fmt"

// Format Cloudwatch metrics can be sent through a metric stream
// with the following output formats:
// - JSON is an open standard human-readable text format.
// - OpenTelemetry 0.7: OpenTelemetry is an open standard binary format.
// - OpenTelemetry 1.0: OpenTelemetry is an open standard binary format.
type Format string

const (
	JSONFormat = "json"
	// OTelFormat is the OpenTelemetry 1.0 format
	OTelFormat = "otel"
)

// GetErrorInvalidFormat is used for testing
func GetErrorInvalidFormat(invalidFormat Format) error {
	return fmt.Errorf(
		"unknown format %q, possible formats are [%s, %s]",
		invalidFormat,
		JSONFormat,
		OTelFormat,
	)
}

func IsFormatValid(format Format) (bool, error) {
	switch format {
	case JSONFormat:
		return true, nil
	case OTelFormat:
		return true, nil
	default:
		return false, GetErrorInvalidFormat(format)
	}
}
