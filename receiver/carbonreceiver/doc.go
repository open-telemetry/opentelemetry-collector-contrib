// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate mdatagen metadata.yaml

// Package carbonreceiver implements a receiver that can be used by the
// OpenTelemetry collector to receive data in the Carbon supported formats.
// Carbon is the backend used by Graphite, see
// https://graphite.readthedocs.io/en/latest/carbon-daemons.html
package carbonreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/carbonreceiver"
