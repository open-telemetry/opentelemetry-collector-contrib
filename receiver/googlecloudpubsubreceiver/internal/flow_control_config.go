// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudpubsubreceiver/internal"

import "time"

type FlowControlConfig struct {
	// The maximum duration the acknowledgement loop waits before sending the acknowledgements.
	TriggerAckBatchDuration time.Duration
	// The ack deadline to use for the Pub/Sub stream.
	StreamAckDeadline time.Duration
	// Pub/Sub flow control settings for the maximum number of outstanding messages.
	MaxOutstandingMessages int64
	// Pub/Sub flow control settings for the maximum number of outstanding bytes.
	MaxOutstandingBytes int64
}

func NewDefaultFlowControlConfig() *FlowControlConfig {
	return &FlowControlConfig{
		TriggerAckBatchDuration: 10 * time.Second,
		StreamAckDeadline:       60 * time.Second,
		MaxOutstandingMessages:  0,
		MaxOutstandingBytes:     0,
	}
}
