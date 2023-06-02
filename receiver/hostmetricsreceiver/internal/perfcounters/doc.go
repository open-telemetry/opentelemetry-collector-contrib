// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package perfcounters is a thin wrapper around
// https://godoc.org/github.com/leoluk/perflib_exporter/perflib that
// provides functions to scrape raw performance counter data, without
// calculating rates or formatting them, from the registry.
package perfcounters // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/perfcounters"
