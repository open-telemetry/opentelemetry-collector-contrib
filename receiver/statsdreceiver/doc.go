// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate mdatagen metadata.yaml

// Package statsdreceiver implements a collector receiver that listens
// on UDP port 8125 by default for incoming StatsD messages and parses
// them into OTLP equivalent metric representations.
package statsdreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/statsdreceiver"
