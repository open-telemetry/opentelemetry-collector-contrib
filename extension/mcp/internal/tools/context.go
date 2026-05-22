// Copyright The OpenTelemetry Authors
// Copyright 2025 Austin Parker
// SPDX-License-Identifier: Apache-2.0

package tools // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/mcp/internal/tools"

import (
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/service"
	"go.opentelemetry.io/collector/service/hostcapabilities"
	"go.uber.org/zap"
)

// ExtensionContext provides access to the extension's capabilities for MCP tools
type ExtensionContext interface {
	// Config access
	GetCollectorConf() *confmap.Conf

	// Component access
	GetHost() component.Host
	GetLogger() *zap.Logger

	// Host capabilities (optional - may return nil)
	GetModuleInfos() *service.ModuleInfos
	GetComponentFactory() hostcapabilities.ComponentFactory

	// Telemetry buffer access
	GetRecentTraces(limit, offset int) []ptrace.Traces
	GetRecentMetrics(limit, offset int) []pmetric.Metrics
	GetRecentLogs(limit, offset int) []plog.Logs
	GetBufferStats() BufferStats
}

// BufferStats mirrors the internal buffer stats
type BufferStats struct {
	TracesCount     int
	TracesCapacity  int
	MetricsCount    int
	MetricsCapacity int
	LogsCount       int
	LogsCapacity    int
}
