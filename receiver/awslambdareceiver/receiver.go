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

func newAWSLambdaReceiver(cfg *Config, set *receiver.Settings, logs consumer.Logs) *awsLambdaReceiver {
	return &awsLambdaReceiver{}
}

func (r *awsLambdaReceiver) Start(_ context.Context, _ component.Host) error {
	return nil
}

func (r *awsLambdaReceiver) Shutdown(_ context.Context) error {
	return nil
}
