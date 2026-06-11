// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metadata

import (
	"context"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

const (
	ReasonPermanent        = "permanent"
	ReasonWrongCode        = "wrong_code"
	ReasonRetryInfoPresent = "retry_info_present"
)

func (t *TelemetryBuilder) RecordRetrySet(delay time.Duration) {
	t.ExtensionResourceExhaustedRetryRetriesSet.Add(context.Background(), 1)
	t.ExtensionResourceExhaustedRetryRetryDelay.Record(context.Background(), delay.Milliseconds())
}

func (t *TelemetryBuilder) RecordRetryNotSet(reason string) {
	t.ExtensionResourceExhaustedRetryRetriesNotSet.Add(context.Background(), 1,
		metric.WithAttributes(attribute.String("reason", reason)))
}
