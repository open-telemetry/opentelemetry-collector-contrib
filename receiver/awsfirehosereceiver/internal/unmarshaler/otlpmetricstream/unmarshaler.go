// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otlpmetricstream // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsfirehosereceiver/internal/unmarshaler/otlpmetricstream"

import (
	"errors"

	"github.com/gogo/protobuf/proto"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/pmetric/pmetricotlp"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsfirehosereceiver/internal/unmarshaler"
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

var _ unmarshaler.MetricsUnmarshaler = (*Unmarshaler)(nil)

// NewUnmarshaler creates a new instance of the Unmarshaler.
func NewUnmarshaler(logger *zap.Logger) *Unmarshaler {
	return &Unmarshaler{logger}
}

// UnmarshalMetrics deserializes the records into pmetric.Metrics
func (u Unmarshaler) UnmarshalMetrics(_ string, records [][]byte) (pmetric.Metrics, error) {
	md := pmetric.NewMetrics()
	for recordIndex, record := range records {
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
				u.logger.Error(
					"Unable to unmarshal input",
					zap.Error(err),
					zap.Int("record_index", recordIndex),
				)
				continue
			}
			req.Metrics().ResourceMetrics().MoveAndAppendTo(md.ResourceMetrics())
		}
	}

	return md, nil
}

// Type of the serialized messages.
func (u Unmarshaler) Type() string {
	return TypeStr
}
