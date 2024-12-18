// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datastream // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/ecs/internal/datastream"

const (
	DataStreamDataset            = "data_stream.dataset"
	DataStreamNamespace          = "data_stream.namespace"
	DataStreamType               = "data_stream.type"
	DefaultDataStreamDataset     = "generic"
	DefaultDataStreamNamespace   = "default"
	DefaultDataStreamTypeLogs    = "logs"
	DefaultDataStreamTypeMetrics = "metrics"
	DefaultDataStreamTypeTraces  = "traces"
)
