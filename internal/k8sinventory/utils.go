// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sinventory // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sinventory"

import (
	"time"

	corev1 "k8s.io/api/core/v1"
)

// GetEventTimestamp returns the EventTimestamp based on the populated k8s event timestamps.
// Priority: EventTime > LastTimestamp > FirstTimestamp.
func GetEventTimestamp(ev *corev1.Event) time.Time {
	var eventTimestamp time.Time

	switch {
	case ev.EventTime.Time != time.Time{}:
		eventTimestamp = ev.EventTime.Time
	case ev.LastTimestamp.Time != time.Time{}:
		eventTimestamp = ev.LastTimestamp.Time
	case ev.FirstTimestamp.Time != time.Time{}:
		eventTimestamp = ev.FirstTimestamp.Time
	}

	return eventTimestamp
}
