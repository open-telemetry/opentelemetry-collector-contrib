// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awss3receiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awss3receiver"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.uber.org/zap"
)

type awss3Receiver struct {
}

func newAWSS3TraceReceiver(_ *Config, _ consumer.Traces, _ *zap.Logger) (*awss3Receiver, error) {
	return &awss3Receiver{}, nil
}

func (r *awss3Receiver) Start(_ context.Context, _ component.Host) error {
	return nil
}

func (r *awss3Receiver) Shutdown(_ context.Context) error {
	return nil
}
