// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package snmpreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/snmpreceiver"

import (
	"context"

	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/snmpreceiver/internal/metadata"
)

type trapCacheTelemetry struct {
	builder *metadata.TelemetryBuilder
	attrs   metric.MeasurementOption
}

func newTrapCacheTelemetry(settings receiver.Settings) (*trapCacheTelemetry, error) {
	builder, err := metadata.NewTelemetryBuilder(settings.TelemetrySettings)
	if err != nil {
		return nil, err
	}
	t := &trapCacheTelemetry{
		builder: builder,
		attrs: metric.WithAttributeSet(attribute.NewSet(
			attribute.String("receiver", settings.ID.String()),
		)),
	}
	t.recordEntries(context.Background(), 0)
	return t, nil
}

// recordEntries receives a snapshot from the cache owner, avoiding concurrent
// reads of the engine maps from metric collection callbacks.
func (t *trapCacheTelemetry) recordEntries(ctx context.Context, entries int) {
	if t == nil {
		return
	}
	t.builder.SnmpTrapsEngineCacheEntries.Record(ctx, int64(entries), t.attrs)
}

func (t *trapCacheTelemetry) recordRejection(ctx context.Context) {
	if t == nil {
		return
	}
	t.builder.SnmpTrapsEngineLimitRejections.Add(ctx, 1, t.attrs)
}

func (t *trapCacheTelemetry) recordExpiration(ctx context.Context, count int) {
	if t == nil || count <= 0 {
		return
	}
	t.builder.SnmpTrapsEngineCacheExpirations.Add(ctx, int64(count), t.attrs)
}
