// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate mdatagen metadata.yaml

// Package gnetexporter implements a high-throughput OTLP exporter that sends
// length-prefixed protobuf frames over a persistent TCP connection managed by
// the gnet event-loop library, bypassing HTTP/2 and gRPC overhead.
package gnetexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/gnetexporter"
