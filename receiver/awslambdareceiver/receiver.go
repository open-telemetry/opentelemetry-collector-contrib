// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awslambdareceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awslambdareceiver"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
)

type awsLambdaReceiver struct{}

func newAWSLambdaReceiver(_ *Config, _ *receiver.Settings, _ consumer.Logs) *awsLambdaReceiver {
	return &awsLambdaReceiver{}
}

func (*awsLambdaReceiver) Start(_ context.Context, _ component.Host) error {
	return nil
}

func (*awsLambdaReceiver) Shutdown(_ context.Context) error {
	return nil
}
