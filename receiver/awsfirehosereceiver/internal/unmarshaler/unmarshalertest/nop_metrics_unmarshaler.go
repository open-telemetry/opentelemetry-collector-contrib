// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package unmarshalertest // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsfirehosereceiver/internal/unmarshaler/unmarshalertest"

import (
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsfirehosereceiver/internal/unmarshaler"
)

const typeStr = "nop"

// NopMetricsUnmarshaler is a MetricsUnmarshaler that doesn't do anything
// with the inputs and just returns the metrics and error passed in.
type NopMetricsUnmarshaler struct {
	metrics pmetric.Metrics
	err     error
}

var _ unmarshaler.MetricsUnmarshaler = (*NopMetricsUnmarshaler)(nil)

// NewNopMetrics provides a nop metrics unmarshaler with the passed
// error and metrics as the result of the unmarshal.
func NewNopMetrics(metrics pmetric.Metrics, err error) *NopMetricsUnmarshaler {
	return &NopMetricsUnmarshaler{metrics: metrics, err: err}
}

// UnmarshalMetrics deserializes the records into metrics.
func (u *NopMetricsUnmarshaler) UnmarshalMetrics([][]byte) (pmetric.Metrics, error) {
	return u.metrics, u.err
}

// Type of the serialized messages.
func (u *NopMetricsUnmarshaler) Type() string {
	return typeStr
}
