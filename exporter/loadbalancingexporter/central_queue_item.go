// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package loadbalancingexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter"

import "errors"

type signalKind string

const (
	signalKindLogs    signalKind = "logs"
	signalKindMetrics signalKind = "metrics"
)

var (
	errCentralQueueFull         = errors.New("central queue is full")
	errCentralQueueInflightFull = errors.New("central queue inflight uncompressed budget is full")
	errCentralQueueStopped      = errors.New("central queue is stopped")
	errCentralQueueItemTooLarge = errors.New("central queue item exceeds max uncompressed batch bytes")
)

type centralQueueItem struct {
	signal              signalKind
	routingKey          []byte
	payload             []byte
	compressedBytes     int
	uncompressedBytes   int
	count               int
	enqueuedAtUnixNano  int64
	attempt             int
	nextAttemptUnixNano int64
}
