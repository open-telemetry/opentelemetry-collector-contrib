// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awscloudwatchmetricstreamsencodingextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awscloudwatchmetricstreamsencodingextension"

import (
	"encoding/binary"
	"errors"

	"go.opentelemetry.io/collector/pdata/pmetric"
)

var errUvarintReadFailure = errors.New("failed to get uvarint from record")

type formatOpenTelemetry10Unmarshaler struct {
	protoUnmarshaler *pmetric.ProtoUnmarshaler
}

var _ pmetric.Unmarshaler = (*formatOpenTelemetry10Unmarshaler)(nil)

func (f formatOpenTelemetry10Unmarshaler) UnmarshalMetrics(record []byte) (pmetric.Metrics, error) {
	// if bytesRead is <= 0, there was an error
	_, bytesRead := binary.Uvarint(record)
	if bytesRead <= 0 {
		return pmetric.Metrics{}, errUvarintReadFailure
	}

	// remove length prefix and get
	// only the protobuf message
	return f.protoUnmarshaler.UnmarshalMetrics(record[bytesRead:])
}
