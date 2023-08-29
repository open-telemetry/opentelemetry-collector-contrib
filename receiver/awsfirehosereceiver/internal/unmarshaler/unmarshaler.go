// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package unmarshaler // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsfirehosereceiver/internal/unmarshaler"

import (
	"go.opentelemetry.io/collector/pdata/pmetric"
)

// MetricsUnmarshaler deserializes the message body
type MetricsUnmarshaler interface {
	// Unmarshal deserializes the records into metrics.
	Unmarshal(records [][]byte) (pmetric.Metrics, error)

	// Type of the serialized messages.
	Type() string
}
