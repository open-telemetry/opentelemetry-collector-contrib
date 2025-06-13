// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package perfcounters is a thin wrapper around code vendored from
// https://pkg.go.dev/github.com/prometheus-community/windows_exporter/pkg/perflib that
// provides functions to scrape raw performance counter data, without
// calculating rates or formatting them, from the registry.
package perfcounters // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/perfcounters"
