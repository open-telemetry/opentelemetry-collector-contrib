// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package facts // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ciscoosreceiver/internal/collectors/facts"

import (
	"context"
	"sync"
	"time"

	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ciscoosreceiver/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ciscoosreceiver/internal/collectors"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ciscoosreceiver/internal/rpc"
)

// Collector implements the Facts collector for Cisco devices
type Collector struct {
	parser        *Parser
	metricBuilder *collectors.MetricBuilder
}

// NewCollector creates a new Facts collector
func NewCollector() *Collector {
	return &Collector{
		parser:        NewParser(),
		metricBuilder: collectors.NewMetricBuilder(),
	}
}

// Name returns the collector name
func (c *Collector) Name() string {
	return "facts"
}

// IsSupported checks if Facts collection is supported on the device
func (c *Collector) IsSupported(client *rpc.Client) bool {
	return client.IsOSSupported("facts")
}

// Collect performs Facts metric collection from the device
func (c *Collector) Collect(ctx context.Context, client *rpc.Client, timestamp time.Time) (pmetric.Metrics, error) {
	metrics := pmetric.NewMetrics()

	// Execute commands concurrently for better performance
	systemInfos := c.collectSystemInfoConcurrently(ctx, client)

	// If no system info collected, try simple facts parsing with version command
	if len(systemInfos) == 0 {
		versionCmd := client.GetCommand("facts_version")
		if versionCmd != "" {
			output, err := client.ExecuteCommand(versionCmd)
			if err == nil {
				if simpleInfo, err := c.parser.ParseSimpleFacts(output); err == nil {
					systemInfos = append(systemInfos, simpleInfo)
				}
			}
		}
	}

	// Merge all system information
	var mergedInfo *SystemInfo
	if len(systemInfos) > 0 {
		mergedInfo = c.parser.MergeSystemInfo(systemInfos...)
	} else {
		// Create default system info for demo
		mergedInfo = NewSystemInfo()
		mergedInfo.Hostname = "cisco-device"
		mergedInfo.UptimeSeconds = 86400 // 1 day default
		mergedInfo.OSType = string(client.GetOSType())
	}

	// Generate metrics
	target := client.GetTarget()
	c.generateSystemMetrics(metrics, mergedInfo, target, timestamp)

	return metrics, nil
}

// collectSystemInfoConcurrently executes multiple commands concurrently for better performance
func (c *Collector) collectSystemInfoConcurrently(ctx context.Context, client *rpc.Client) []*SystemInfo {
	type commandResult struct {
		name   string
		output string
		err    error
	}

	// Define commands to execute
	commands := map[string]string{
		"version": client.GetCommand("facts_version"),
		"memory":  client.GetCommand("facts_memory"),
		"cpu":     client.GetCommand("facts_cpu"),
	}

	// Execute commands concurrently
	var wg sync.WaitGroup
	results := make(chan commandResult, len(commands))

	for name, cmd := range commands {
		if cmd == "" {
			continue // Skip unsupported commands
		}

		wg.Add(1)
		go func(cmdName, command string) {
			defer wg.Done()
			output, err := client.ExecuteCommand(command)
			results <- commandResult{name: cmdName, output: output, err: err}
		}(name, cmd)
	}

	// Close results channel when all goroutines complete
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results and parse
	var systemInfos []*SystemInfo
	for result := range results {
		if result.err != nil {
			continue // Skip failed commands
		}

		switch result.name {
		case "version":
			if versionInfo, err := c.parser.ParseVersion(result.output); err == nil {
				systemInfos = append(systemInfos, versionInfo)
			}
		case "memory":
			if memoryInfo, err := c.parser.ParseMemory(result.output); err == nil {
				systemInfos = append(systemInfos, memoryInfo)
			}
		case "cpu":
			if cpuInfo, err := c.parser.ParseCPU(result.output); err == nil {
				systemInfos = append(systemInfos, cpuInfo)
			}
		}
	}

	return systemInfos
}

// generateSystemMetrics creates OpenTelemetry metrics for system information
func (c *Collector) generateSystemMetrics(metrics pmetric.Metrics, sysInfo *SystemInfo, target string, timestamp time.Time) {
	// Common attributes for all facts metrics
	baseAttributes := map[string]string{
		"target": target,
	}

	// Add hostname if available
	if sysInfo.Hostname != "" {
		baseAttributes["hostname"] = sysInfo.Hostname
	}

	// Add version if available
	if sysInfo.Version != "" {
		baseAttributes["version"] = sysInfo.Version
	}

	// Add model if available
	if sysInfo.Model != "" {
		baseAttributes["model"] = sysInfo.Model
	}

	// cisco_exporter compatible version metric
	if sysInfo.Version != "" {
		// Optimize: reuse baseAttributes and add version directly
		baseAttributes["version"] = sysInfo.Version
		c.metricBuilder.CreateGaugeMetric(
			metrics,
			internal.MetricPrefix+"facts_version",
			"Running OS version",
			"1",
			int64(1), // Always 1 to indicate version is available
			timestamp,
			baseAttributes,
		)
		delete(baseAttributes, "version") // Clean up for next metrics
	}

	// cisco_exporter compatible memory metrics
	if sysInfo.IsMemoryInfoAvailable() {
		// Optimize: reuse baseAttributes and modify type attribute
		baseAttributes["type"] = "total"
		c.metricBuilder.CreateGaugeMetric(
			metrics,
			internal.MetricPrefix+"facts_memory_total",
			"Total memory",
			"bytes",
			sysInfo.MemoryTotal,
			timestamp,
			baseAttributes,
		)

		baseAttributes["type"] = "used"
		c.metricBuilder.CreateGaugeMetric(
			metrics,
			internal.MetricPrefix+"facts_memory_used",
			"Used memory",
			"bytes",
			sysInfo.MemoryUsed,
			timestamp,
			baseAttributes,
		)

		if sysInfo.MemoryFree > 0 {
			baseAttributes["type"] = "free"
			c.metricBuilder.CreateGaugeMetric(
				metrics,
				internal.MetricPrefix+"facts_memory_free",
				"Free memory",
				"bytes",
				sysInfo.MemoryFree,
				timestamp,
				baseAttributes,
			)
		}
		delete(baseAttributes, "type") // Clean up for next metrics
	}

	// cisco_exporter compatible detailed CPU metrics
	if sysInfo.IsDetailedCPUInfoAvailable() {
		// Optimize: reuse baseAttributes for all CPU metrics
		if sysInfo.CPUFiveSecondsPercent > 0 {
			c.metricBuilder.CreateGaugeMetric(
				metrics,
				internal.MetricPrefix+"facts_cpu_five_seconds_percent",
				"CPU utilization for five seconds",
				"percent",
				int64(sysInfo.CPUFiveSecondsPercent),
				timestamp,
				baseAttributes,
			)
		}

		if sysInfo.CPUOneMinutePercent > 0 {
			c.metricBuilder.CreateGaugeMetric(
				metrics,
				internal.MetricPrefix+"facts_cpu_one_minute_percent",
				"CPU utilization for one minute",
				"percent",
				int64(sysInfo.CPUOneMinutePercent),
				timestamp,
				baseAttributes,
			)
		}

		if sysInfo.CPUFiveMinutesPercent > 0 {
			c.metricBuilder.CreateGaugeMetric(
				metrics,
				internal.MetricPrefix+"facts_cpu_five_minutes_percent",
				"CPU utilization for five minutes",
				"percent",
				int64(sysInfo.CPUFiveMinutesPercent),
				timestamp,
				baseAttributes,
			)
		}

		if sysInfo.CPUInterruptPercent > 0 {
			c.metricBuilder.CreateGaugeMetric(
				metrics,
				internal.MetricPrefix+"facts_cpu_interrupt_percent",
				"Interrupt percentage",
				"percent",
				int64(sysInfo.CPUInterruptPercent),
				timestamp,
				baseAttributes,
			)
		}
	}
}

// GetMetricNames returns the names of metrics this collector generates
func (c *Collector) GetMetricNames() []string {
	return []string{
		// cisco_exporter compatible metrics only
		internal.MetricPrefix + "facts_version",
		internal.MetricPrefix + "facts_memory_total",
		internal.MetricPrefix + "facts_memory_used",
		internal.MetricPrefix + "facts_memory_free",
		internal.MetricPrefix + "facts_cpu_five_seconds_percent",
		internal.MetricPrefix + "facts_cpu_one_minute_percent",
		internal.MetricPrefix + "facts_cpu_five_minutes_percent",
		// Note: cisco_facts_cpu_interrupt_percent is defined but not exposed in cisco_exporter Describe method
	}
}

// GetRequiredCommands returns the commands this collector needs to execute
func (c *Collector) GetRequiredCommands() []string {
	return []string{
		"show version",
		"show memory statistics",
		"show processes cpu",
	}
}
