// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate mdatagen metadata.yaml

// Package gnetreceiver implements a high-throughput OTLP receiver using the
// gnet event-loop network library instead of the standard gRPC stack.
// It accepts length-prefixed protobuf frames over raw TCP and converts them
// into pdata signals for downstream consumers.
package gnetreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/gnetreceiver"
