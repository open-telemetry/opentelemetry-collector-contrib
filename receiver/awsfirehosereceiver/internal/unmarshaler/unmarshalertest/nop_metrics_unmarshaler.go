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

// NewNopMetrics provides a nop metrics unmarshaler with the default
// pmetric.Metrics and no error.
func NewNopMetrics() *NopMetricsUnmarshaler {
	return &NopMetricsUnmarshaler{}
}

// NewWithMetrics provides a nop metrics unmarshaler with the passed
// in metrics as the result of the UnmarshalMetrics and no error.
func NewWithMetrics(metrics pmetric.Metrics) *NopMetricsUnmarshaler {
	return &NopMetricsUnmarshaler{metrics: metrics}
}

// NewErrMetrics provides a nop metrics unmarshaler with the passed
// in error as the UnmarshalMetrics error.
func NewErrMetrics(err error) *NopMetricsUnmarshaler {
	return &NopMetricsUnmarshaler{err: err}
}

// UnmarshalMetrics deserializes the records into metrics.
func (u *NopMetricsUnmarshaler) UnmarshalMetrics(string, [][]byte) (pmetric.Metrics, error) {
	return u.metrics, u.err
}

// Type of the serialized messages.
func (u *NopMetricsUnmarshaler) Type() string {
	return typeStr
}
