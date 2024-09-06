// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package deltatocumulativeprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/processor"
)

var _ processor.Metrics = Chain(nil)

// Chain calls processors in series.
// They must be manually setup so that their ConsumeMetrics() invoke each other
type Chain []processor.Metrics

func (c Chain) Capabilities() consumer.Capabilities {
	if len(c) == 0 {
		return consumer.Capabilities{}
	}
	return c[0].Capabilities()
}

func (c Chain) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	if len(c) == 0 {
		return nil
	}
	return c[0].ConsumeMetrics(ctx, md)
}

func (c Chain) Shutdown(ctx context.Context) error {
	for _, proc := range c {
		if err := proc.Shutdown(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (c Chain) Start(ctx context.Context, host component.Host) error {
	for _, proc := range c {
		if err := proc.Start(ctx, host); err != nil {
			return err
		}
	}
	return nil
}
