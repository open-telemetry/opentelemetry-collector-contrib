// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package macosunifiedloggingreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/macosunifiedloggingreceiver"

import (
	"time"
)

// Config defines configuration for the macOS unified logging receiver
// Separated into a common file that isn't platform specific so that factory_others.go can reference it
type Config struct {
	// ArchivePath is a path or glob pattern to .logarchive directory(ies)
	// If empty, reads from the live system logs
	// Supports glob patterns (e.g., "*.logarchive", "**/logs/*.logarchive")
	ArchivePath string `mapstructure:"archive_path"`

	// resolvedArchivePaths stores the expanded archive paths after glob resolution
	// This is populated during Validate() and not exposed to users
	// Only used on darwin platform (see config.go)
	resolvedArchivePaths []string

	// Predicate is a filter predicate to pass to the log command
	// Example: "subsystem == 'com.apple.systempreferences'"
	Predicate string `mapstructure:"predicate"`

	// StartTime specifies when to start reading logs from
	// Format: "2006-01-02 15:04:05"
	StartTime string `mapstructure:"start_time"`

	// EndTime specifies when to stop reading logs
	// Only used with archive_path
	EndTime string `mapstructure:"end_time"`

	// MaxPollInterval specifies the maximum interval between polling for new logs (live mode only)
	// The actual poll interval uses exponential backoff based on whether logs are being actively written
	MaxPollInterval time.Duration `mapstructure:"max_poll_interval"`

	// MaxLogAge specifies the maximum age of logs to read on startup
	// Only applies to live mode. Format: "24h", "1h30m", etc.
	MaxLogAge time.Duration `mapstructure:"max_log_age"`

	// Format specifies the output format from the log command
	// Options: "default" (system default), "ndjson", "json", "syslog", "compact"
	// Default: "default"
	Format string `mapstructure:"format"`

	// prevent unkeyed literal initialization
	_ struct{}
}
