// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package queries

// QPM (Query Performance Monitoring) configuration constants
const (
	// Default values used when configuration is not specified
	DefaultQueryMonitoringResponseTimeThreshold = 500 // milliseconds
	DefaultQueryMonitoringCountThreshold        = 20  // queries

	// Validation ranges for configuration values
	MinQueryMonitoringResponseTimeThreshold = 1    // minimum realistic threshold for Oracle
	MaxQueryMonitoringResponseTimeThreshold = 5000 // practical limit for OLTP workloads
	MinQueryMonitoringCountThreshold        = 10   // prevents too little data collection
	MaxQueryMonitoringCountThreshold        = 50   // performance limit for concurrent queries
)
