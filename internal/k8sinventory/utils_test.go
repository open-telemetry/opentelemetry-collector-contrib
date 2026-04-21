// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sinventory // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sinventory"

import (
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGetEventTimestamp(t *testing.T) {
	now := time.Now()
	eventTime := now.Add(time.Hour)
	lastTimestamp := now.Add(30 * time.Minute)
	firstTimestamp := now.Add(15 * time.Minute)

	tests := []struct {
		name      string
		event     *corev1.Event
		want      time.Time
		wantEmpty bool
	}{
		{
			name: "EventTime is set - highest priority",
			event: &corev1.Event{
				EventTime: metav1.MicroTime{Time: eventTime},
			},
			want: eventTime,
		},
		{
			name: "EventTime is set with other timestamps - EventTime takes priority",
			event: &corev1.Event{
				EventTime:      metav1.MicroTime{Time: eventTime},
				LastTimestamp:  metav1.Time{Time: lastTimestamp},
				FirstTimestamp: metav1.Time{Time: firstTimestamp},
			},
			want: eventTime,
		},
		{
			name: "Only LastTimestamp is set",
			event: &corev1.Event{
				LastTimestamp: metav1.Time{Time: lastTimestamp},
			},
			want: lastTimestamp,
		},
		{
			name: "LastTimestamp and FirstTimestamp are set - LastTimestamp takes priority",
			event: &corev1.Event{
				LastTimestamp:  metav1.Time{Time: lastTimestamp},
				FirstTimestamp: metav1.Time{Time: firstTimestamp},
			},
			want: lastTimestamp,
		},
		{
			name: "Only FirstTimestamp is set",
			event: &corev1.Event{
				FirstTimestamp: metav1.Time{Time: firstTimestamp},
			},
			want: firstTimestamp,
		},
		{
			name:      "No timestamps are set - returns zero time",
			event:     &corev1.Event{},
			wantEmpty: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetEventTimestamp(tt.event)
			if tt.wantEmpty {
				if !got.IsZero() {
					t.Errorf("GetEventTimestamp() = %v, want zero time", got)
				}
			} else {
				if !got.Equal(tt.want) {
					t.Errorf("GetEventTimestamp() = %v, want %v", got, tt.want)
				}
			}
		})
	}
}
