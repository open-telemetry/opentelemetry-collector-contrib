// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package firehoselog // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsfirehosereceiver/internal/unmarshaler/firehoselog"

type firehoseLog struct {
	Timestamp   int64  `json:"timestamp"`
	FirehoseARN string `json:"firehoseARN"`
	Message     string `json:"message"`
}
