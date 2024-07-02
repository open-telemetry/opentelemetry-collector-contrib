// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsapplicationsignalsprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/awsapplicationsignalsprocessor"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

type awsapplicationsignalsprocessor struct{}

func (ap *awsapplicationsignalsprocessor) StartMetrics(_ context.Context, _ component.Host) error {
	return nil
}

func (ap *awsapplicationsignalsprocessor) StartTraces(_ context.Context, _ component.Host) error {
	return nil
}

func (ap *awsapplicationsignalsprocessor) Shutdown(_ context.Context) error {
	return nil
}

func (ap *awsapplicationsignalsprocessor) processTraces(_ context.Context, td ptrace.Traces) (ptrace.Traces, error) {
	return td, nil
}

func (ap *awsapplicationsignalsprocessor) processMetrics(_ context.Context, md pmetric.Metrics) (pmetric.Metrics, error) {
	return md, nil
}
