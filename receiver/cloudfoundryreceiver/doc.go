// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate mdatagen metadata.yaml

// Package cloudfoundryreceiver implements a receiver that can be used by the
// Opentelemetry collector to receive Cloud Foundry metrics via its Reverse
// Log Proxy (RLP) Gateway component. The protocol is handled by the
// go-loggregator library, which uses HTTP to connect to the gateway and receive
// JSON-protobuf encoded v2 Envelope messages as documented by loggregator-api.
package cloudfoundryreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/cloudfoundryreceiver"
