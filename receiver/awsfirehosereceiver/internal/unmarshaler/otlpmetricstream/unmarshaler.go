// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otlpmetricstream // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsfirehosereceiver/internal/unmarshaler/otlpmetricstream"

import (
	"errors"
	"fmt"

	"github.com/gogo/protobuf/proto"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/pmetric/pmetricotlp"
	"go.uber.org/zap"
)

const (
	// Supported version depends on version of go.opentelemetry.io/collector/pdata/pmetric/pmetricotlp dependency
	TypeStr = "otlp_v1"
)

var errInvalidOTLPFormatStart = errors.New("unable to decode data length from message")

// Unmarshaler for the CloudWatch Metric Stream OpenTelemetry record format.
//
// More details can be found at:
// https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/CloudWatch-metric-streams-formats-opentelemetry-100.html
type Unmarshaler struct {
	logger *zap.Logger
}

var _ pmetric.Unmarshaler = (*Unmarshaler)(nil)

// NewUnmarshaler creates a new instance of the Unmarshaler.
func NewUnmarshaler(logger *zap.Logger) *Unmarshaler {
	return &Unmarshaler{logger}
}

// UnmarshalMetrics deserializes the recordsas a length-delimited sequence of
// OTLP metrics into pmetric.Metrics.
func (u Unmarshaler) UnmarshalMetrics(record []byte) (pmetric.Metrics, error) {
	md := pmetric.NewMetrics()
	dataLen, pos := len(record), 0
	for pos < dataLen {
		n, nLen := proto.DecodeVarint(record)
		if nLen == 0 && n == 0 {
			return md, errInvalidOTLPFormatStart
		}
		req := pmetricotlp.NewExportRequest()
		pos += nLen
		err := req.UnmarshalProto(record[pos : pos+int(n)])
		pos += int(n)
		if err != nil {
			return pmetric.Metrics{}, fmt.Errorf("unable to unmarshal input: %w", err)
		}
		req.Metrics().ResourceMetrics().MoveAndAppendTo(md.ResourceMetrics())
	}

	return md, nil
}

// Type of the serialized messages.
func (u Unmarshaler) Type() string {
	return TypeStr
}
