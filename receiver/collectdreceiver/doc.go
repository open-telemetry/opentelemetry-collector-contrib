// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate mdatagen metadata.yaml

// Package collectdreceiver implements a receiver that can be used by the
// Opentelemetry collector to receive traces from CollectD http_write plugin
// in JSON format.
package collectdreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/collectdreceiver"
