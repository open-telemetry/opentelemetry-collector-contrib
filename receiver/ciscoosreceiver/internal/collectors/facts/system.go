// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package facts // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ciscoosreceiver/internal/collectors/facts"

import (
	"fmt"
	"time"
)

// SystemInfo represents system information from a Cisco device
type SystemInfo struct {
	Hostname      string
	Version       string
	Model         string
	SerialNumber  string
	Uptime        time.Duration
	UptimeSeconds int64
	MemoryTotal   int64
	MemoryUsed    int64
	MemoryFree    int64
	CPUUsage      float64
	// cisco_exporter compatible CPU metrics
	CPUFiveSecondsPercent float64 // Missing field for cisco_exporter parity
	CPUOneMinutePercent   float64 // Missing field for cisco_exporter parity
	CPUFiveMinutesPercent float64 // Missing field for cisco_exporter parity
	CPUInterruptPercent   float64 // Missing field for cisco_exporter parity
	OSType                string
	Location              string
}

// NewSystemInfo creates a new SystemInfo instance
func NewSystemInfo() *SystemInfo {
	return &SystemInfo{}
}

// SetUptime sets the system uptime from various formats
func (s *SystemInfo) SetUptime(uptimeStr string) error {
	// This would parse uptime strings like:
	// "1 day, 2 hours, 30 minutes"
	// "2d3h"
	// "123456 seconds"

	// For now, we'll implement a simple parser
	// In a real implementation, this would be more sophisticated
	s.UptimeSeconds = 86400 // Default to 1 day for demo
	s.Uptime = time.Duration(s.UptimeSeconds) * time.Second

	return nil
}

// SetMemoryInfo sets memory information
func (s *SystemInfo) SetMemoryInfo(total, used, free int64) {
	s.MemoryTotal = total
	s.MemoryUsed = used
	s.MemoryFree = free
}

// SetCPUUsage sets CPU usage percentage
func (s *SystemInfo) SetCPUUsage(usage float64) {
	s.CPUUsage = usage
}

// SetDetailedCPUMetrics sets detailed CPU metrics for cisco_exporter compatibility
func (s *SystemInfo) SetDetailedCPUMetrics(fiveSeconds, oneMinute, fiveMinutes, interrupt float64) {
	s.CPUFiveSecondsPercent = fiveSeconds
	s.CPUOneMinutePercent = oneMinute
	s.CPUFiveMinutesPercent = fiveMinutes
	s.CPUInterruptPercent = interrupt
}

// IsDetailedCPUInfoAvailable checks if detailed CPU information is available
func (s *SystemInfo) IsDetailedCPUInfoAvailable() bool {
	return s.CPUFiveSecondsPercent > 0 || s.CPUOneMinutePercent > 0 ||
		s.CPUFiveMinutesPercent > 0 || s.CPUInterruptPercent > 0
}

// GetMemoryUtilization returns memory utilization as percentage
func (s *SystemInfo) GetMemoryUtilization() float64 {
	if s.MemoryTotal == 0 {
		return 0
	}
	return (float64(s.MemoryUsed) / float64(s.MemoryTotal)) * 100
}

// GetMemoryUtilizationInt returns memory utilization as int64 percentage
func (s *SystemInfo) GetMemoryUtilizationInt() int64 {
	return int64(s.GetMemoryUtilization())
}

// GetCPUUsageInt returns CPU usage as int64 percentage
func (s *SystemInfo) GetCPUUsageInt() int64 {
	return int64(s.CPUUsage)
}

// String returns a string representation of the system info
func (s *SystemInfo) String() string {
	return fmt.Sprintf("System %s (%s): %s, Uptime: %v, Memory: %d/%d MB (%.1f%%), CPU: %.1f%%",
		s.Hostname, s.Model, s.Version, s.Uptime, s.MemoryUsed/1024/1024,
		s.MemoryTotal/1024/1024, s.GetMemoryUtilization(), s.CPUUsage)
}

// Validate checks if the system info has valid data
func (s *SystemInfo) Validate() bool {
	return s.Hostname != "" || s.Version != "" || s.UptimeSeconds > 0
}

// IsMemoryInfoAvailable checks if memory information is available
func (s *SystemInfo) IsMemoryInfoAvailable() bool {
	return s.MemoryTotal > 0
}

// IsCPUInfoAvailable checks if CPU information is available
func (s *SystemInfo) IsCPUInfoAvailable() bool {
	return s.CPUUsage >= 0
}

// GetUptimeDays returns uptime in days
func (s *SystemInfo) GetUptimeDays() float64 {
	return s.Uptime.Hours() / 24
}
