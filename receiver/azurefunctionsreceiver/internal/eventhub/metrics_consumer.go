// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package eventhub // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azurefunctionsreceiver/internal/eventhub"

import (
	"context"
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azurefunctionsreceiver/internal/trigger"
)

// MetricsConsumer is a trigger.Consumer that unmarshals each Content message as metrics.
type MetricsConsumer struct {
	unmarshaler pmetric.Unmarshaler
	nextMetrics consumer.Metrics
}

// NewMetricsConsumer returns a trigger.Consumer for Event Hub metrics bindings.
func NewMetricsConsumer(unmarshaler pmetric.Unmarshaler, nextMetrics consumer.Metrics) *MetricsConsumer {
	return &MetricsConsumer{
		unmarshaler: unmarshaler,
		nextMetrics: nextMetrics,
	}
}

// ConsumeEvents implements trigger.Consumer.
func (c *MetricsConsumer) ConsumeEvents(ctx context.Context, req trigger.ParsedRequest) error {
	merged := pmetric.NewMetrics()
	for i, msg := range req.Content {
		metrics, err := c.unmarshaler.UnmarshalMetrics(msg)
		if err != nil {
			return fmt.Errorf("unmarshal message %d: %w", i, err)
		}
		if metrics.DataPointCount() == 0 {
			continue
		}
		if len(req.Metadata) > 0 {
			trigger.AddMetadataToMetrics(&metrics, req.Metadata)
		}
		for j := 0; j < metrics.ResourceMetrics().Len(); j++ {
			metrics.ResourceMetrics().At(j).CopyTo(merged.ResourceMetrics().AppendEmpty())
		}
	}
	if merged.DataPointCount() == 0 {
		// Same policy as logs: requests that produce no data points are rejected
		// as permanent errors (anomaly or bad payload).
		return errors.New("no metrics to consume")
	}
	return c.nextMetrics.ConsumeMetrics(ctx, merged)
}
