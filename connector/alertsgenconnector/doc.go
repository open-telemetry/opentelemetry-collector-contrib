// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate mdatagen metadata.yaml

// Package alertsgenconnector implements a tri-signal sliding-window alert evaluator
// with ns-precision time, TSDB-based state sync, dedup, storm prevention, and
// AlertManager notifications.
package alertsgenconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/alertsgenconnector"
