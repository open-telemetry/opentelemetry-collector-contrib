// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// windowsperfcountersreceiver implements a collector receiver that
// collects the configured performance counter data at the configured
// collection interval and converts them into OTLP equivalent metric
// representations.
//

//go:generate mdatagen metadata.yaml

// This receiver is only compatible with Windows.
package windowsperfcountersreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/windowsperfcountersreceiver"
