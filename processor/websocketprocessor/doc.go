// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate mdatagen metadata.yaml

// Package websocketprocessor can be positioned anywhere in a pipeline, allowing
// data to pass through to the next component. Simultaneously, it makes a portion
// of the data accessible to WebSocket clients connecting on a configurable port.
package websocketprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/websocketprocessor"
