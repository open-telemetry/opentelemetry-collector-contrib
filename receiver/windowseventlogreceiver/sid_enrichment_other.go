// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package windowseventlogreceiver

import (
	"go.opentelemetry.io/collector/consumer"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/windowseventlogreceiver/internal/sidcache"
)

// newSIDEnrichingConsumer is a stub for non-Windows platforms
// This allows the code to compile on macOS/Linux during development
//
//nolint:unused // stub for non-Windows platforms
func newSIDEnrichingConsumer(next consumer.Logs, _ sidcache.Cache, _ *zap.Logger) consumer.Logs {
	// SID enrichment is only supported on Windows
	// Return the next consumer unchanged
	return next
}
