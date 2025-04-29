// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otlpmetricstream // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsfirehosereceiver/internal/unmarshaler/otlpmetricstream"

import (
	"context"
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awscloudwatchmetricstreamsencodingextension"
)

const (
	// TypeStr Supported version depends on version of go.opentelemetry.io/collector/pdata/pmetric/pmetricotlp dependency
	TypeStr = "otlp_v1"

	// format of the aws cloudwatch metric stream encoding extension
	format = "opentelemetry1.0"
)

// Unmarshaler for the CloudWatch Metric Stream OpenTelemetry record format.
//
// More details can be found at:
// https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/CloudWatch-metric-streams-formats-opentelemetry-100.html
type Unmarshaler struct {
	unmarshaler pmetric.Unmarshaler
}

var _ pmetric.Unmarshaler = (*Unmarshaler)(nil)

// NewUnmarshaler creates a new instance of the aws cloudwatch metric streams
// encoding extension unmarshaler for the OpenTelemetry record format.
func NewUnmarshaler(ctx context.Context) (pmetric.Unmarshaler, error) {
	f := awscloudwatchmetricstreamsencodingextension.NewFactory()
	extensionID := component.NewID(f.Type())
	ext, err := f.Create(ctx, extension.Settings{
		ID: extensionID,
	}, &awscloudwatchmetricstreamsencodingextension.Config{
		Format: format,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create encoding extension for %q format: %w", TypeStr, err)
	}
	unmarshaler, ok := ext.(pmetric.Unmarshaler)
	if !ok {
		return nil, errors.New("unexpected: failed to cast aws cloudwatch metric streams encoding extension to unmarshaler")
	}
	return &Unmarshaler{unmarshaler: unmarshaler}, nil
}

// UnmarshalMetrics uses the unmarshal from the encoding extension to
// get the resource metrics from the record
func (u *Unmarshaler) UnmarshalMetrics(record []byte) (pmetric.Metrics, error) {
	return u.unmarshaler.UnmarshalMetrics(record)
}
