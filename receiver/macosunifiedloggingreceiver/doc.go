// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate make mdatagen

// Package macosunifiedloggingreceiver implements a receiver that uses the native
// macOS `log` command to retrieve and parse unified logging data.
// It supports both live system logs and archived log files (.logarchive).
package macosunifiedloggingreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/macosunifiedloggingreceiver"
