// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awscloudwatchmetricstreamsencodingextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awscloudwatchmetricstreamsencodingextension"

import (
	"fmt"

	"go.opentelemetry.io/collector/pdata/pmetric"
)

type formatOpenTelemetry10Unmarshaler struct{}

var _ pmetric.Unmarshaler = (*formatOpenTelemetry10Unmarshaler)(nil)

func (f formatOpenTelemetry10Unmarshaler) UnmarshalMetrics(_ []byte) (pmetric.Metrics, error) {
	return pmetric.Metrics{}, fmt.Errorf("UnmarshalMetrics unimplemented for format %q", formatOpenTelemetry10)
}
