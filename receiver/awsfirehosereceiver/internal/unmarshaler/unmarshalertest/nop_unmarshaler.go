// Copyright  The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
// in metrics as the result of the Unmarshal and no error.
func NewWithMetrics(metrics pmetric.Metrics) *NopMetricsUnmarshaler {
	return &NopMetricsUnmarshaler{metrics: metrics}
}

// NewErrMetrics provides a nop metrics unmarshaler with the passed
// in error as the Unmarshal error.
func NewErrMetrics(err error) *NopMetricsUnmarshaler {
	return &NopMetricsUnmarshaler{err: err}
}

// Unmarshal deserializes the records into metrics.
func (u *NopMetricsUnmarshaler) Unmarshal([][]byte) (pmetric.Metrics, error) {
	return u.metrics, u.err
}

// Type of the serialized messages.
func (u *NopMetricsUnmarshaler) Type() string {
	return typeStr
}
