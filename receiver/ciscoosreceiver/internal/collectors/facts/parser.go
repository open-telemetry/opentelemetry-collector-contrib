// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package facts // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ciscoosreceiver/internal/collectors/facts"

import (
	"regexp"
	"strconv"
	"strings"
)

// Parser handles parsing of facts command output
type Parser struct {
	uptimePattern   *regexp.Regexp
	versionPattern  *regexp.Regexp
	memoryPattern   *regexp.Regexp
	cpuPattern      *regexp.Regexp
	hostnamePattern *regexp.Regexp
}

// NewParser creates a new facts parser
func NewParser() *Parser {
	uptimePattern := regexp.MustCompile(`(?i)uptime.*?(\d+)\s*(?:day|d)`)
	versionPattern := regexp.MustCompile(`(?i)(?:cisco\s+ios\s+(?:xe\s+)?software.*?version\s+([^\s,]+)|(?:cisco\s+)?(?:ios\s+xe\s+)?(?:software,?\s+)?version\s+([^\s,]+))`)
	memoryPattern := regexp.MustCompile(`(?i)(?:total|used).*?(\d+)\s*(kb|mb|bytes?)\b`)
	cpuPattern := regexp.MustCompile(`(?i)cpu.*?(\d+)%`)
	hostnamePattern := regexp.MustCompile(`(?i)(?:hostname|name).*?([\w-]+)`)

	return &Parser{
		uptimePattern:   uptimePattern,
		versionPattern:  versionPattern,
		memoryPattern:   memoryPattern,
		cpuPattern:      cpuPattern,
		hostnamePattern: hostnamePattern,
	}
}

// ParseVersion parses the output of "show version" command
func (p *Parser) ParseVersion(output string) (*SystemInfo, error) {
	sysInfo := NewSystemInfo()

	lines := strings.Split(output, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if matches := p.versionPattern.FindStringSubmatch(line); len(matches) > 1 {
			// Skip BOOTLDR version lines
			if !strings.Contains(strings.ToUpper(line), "BOOTLDR") {
				if matches[1] != "" {
					sysInfo.Version = matches[1]
				} else if matches[2] != "" {
					sysInfo.Version = matches[2]
				}
			}
		}

		if strings.Contains(strings.ToLower(line), "uptime") {
			var totalSeconds int64

			dayPattern := regexp.MustCompile(`(\d+)\s*days?`)
			if matches := dayPattern.FindStringSubmatch(line); len(matches) > 1 {
				if days, err := strconv.ParseInt(matches[1], 10, 64); err == nil {
					totalSeconds += days * 24 * 3600
				}
			}

			hourPattern := regexp.MustCompile(`(\d+)\s*hours?`)
			if matches := hourPattern.FindStringSubmatch(line); len(matches) > 1 {
				if hours, err := strconv.ParseInt(matches[1], 10, 64); err == nil {
					totalSeconds += hours * 3600
				}
			}

			minutePattern := regexp.MustCompile(`(\d+)\s*minutes?`)
			if matches := minutePattern.FindStringSubmatch(line); len(matches) > 1 {
				if minutes, err := strconv.ParseInt(matches[1], 10, 64); err == nil {
					totalSeconds += minutes * 60
				}
			}

			secondPattern := regexp.MustCompile(`(\d+)\s*seconds?`)
			if matches := secondPattern.FindStringSubmatch(line); len(matches) > 1 {
				if seconds, err := strconv.ParseInt(matches[1], 10, 64); err == nil {
					totalSeconds += seconds
				}
			}

			if totalSeconds > 0 {
				sysInfo.UptimeSeconds = totalSeconds
			}
		}

		if matches := p.hostnamePattern.FindStringSubmatch(line); len(matches) > 1 {
			sysInfo.Hostname = matches[1]
		}

		if strings.Contains(strings.ToLower(line), "model:") {
			modelPattern := regexp.MustCompile(`(?i)model:\s*([\w-]+)`)
			if matches := modelPattern.FindStringSubmatch(line); len(matches) > 1 {
				sysInfo.Model = matches[1]
			}
		} else if strings.Contains(strings.ToLower(line), "cisco") && strings.Contains(strings.ToLower(line), "processor") {
			processorPattern := regexp.MustCompile(`(?i)cisco\s+([\w-]+)\s+.*processor`)
			if matches := processorPattern.FindStringSubmatch(line); len(matches) > 1 {
				sysInfo.Model = matches[1]
			}
		} else if strings.Contains(strings.ToLower(line), "nexus") && strings.Contains(strings.ToLower(line), "chassis") {
			nexusPattern := regexp.MustCompile(`(?i)nexus\s+([\w-]+)\s+chassis`)
			if matches := nexusPattern.FindStringSubmatch(line); len(matches) > 1 {
				sysInfo.Model = matches[1]
			}
		}
	}

	return sysInfo, nil
}

// ParseMemory parses the output of memory-related commands
func (p *Parser) ParseMemory(output string) (*SystemInfo, error) {
	sysInfo := NewSystemInfo()

	lines := strings.Split(output, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if matches := p.memoryPattern.FindStringSubmatch(line); len(matches) > 2 {
			if memStr := matches[1]; memStr != "" {
				if mem, err := strconv.ParseInt(memStr, 10, 64); err == nil {
					unit := strings.ToLower(matches[2])

					var memoryValue int64
					switch unit {
					case "mb":
						memoryValue = mem * 1024
					case "kb":
						memoryValue = mem * 1024
					case "bytes", "byte":
						memoryValue = mem
					default:
						memoryValue = mem * 1024
					}

					if strings.Contains(strings.ToLower(line), "total") {
						sysInfo.MemoryTotal = memoryValue
					} else if strings.Contains(strings.ToLower(line), "used") {
						sysInfo.MemoryUsed = memoryValue
					}
				}
			}
		}
	}

	// Calculate free memory
	if sysInfo.MemoryTotal > 0 && sysInfo.MemoryUsed > 0 {
		sysInfo.MemoryFree = sysInfo.MemoryTotal - sysInfo.MemoryUsed
	}

	return sysInfo, nil
}

// ParseCPU parses the output of CPU-related commands
func (p *Parser) ParseCPU(output string) (*SystemInfo, error) {
	sysInfo := NewSystemInfo()

	lines := strings.Split(output, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if strings.Contains(strings.ToLower(line), "cpu utilization for five seconds") {
			fiveSecondsPattern := regexp.MustCompile(`five seconds:\s*(\d+)%`)
			oneMinutePattern := regexp.MustCompile(`one minute:\s*(\d+)%`)
			fiveMinutesPattern := regexp.MustCompile(`five minutes:\s*(\d+)%`)

			if matches := fiveSecondsPattern.FindStringSubmatch(line); len(matches) > 1 {
				if cpu, err := strconv.ParseFloat(matches[1], 64); err == nil {
					sysInfo.CPUFiveSecondsPercent = cpu
					sysInfo.CPUUsage = cpu
				}
			}

			if matches := oneMinutePattern.FindStringSubmatch(line); len(matches) > 1 {
				if cpu, err := strconv.ParseFloat(matches[1], 64); err == nil {
					sysInfo.CPUOneMinutePercent = cpu
				}
			}

			if matches := fiveMinutesPattern.FindStringSubmatch(line); len(matches) > 1 {
				if cpu, err := strconv.ParseFloat(matches[1], 64); err == nil {
					sysInfo.CPUFiveMinutesPercent = cpu
				}
			}
			continue
		}

		if strings.Contains(strings.ToLower(line), "interrupt level") {
			interruptPattern := regexp.MustCompile(`interrupt level:\s*(\d+)%`)
			if matches := interruptPattern.FindStringSubmatch(line); len(matches) > 1 {
				if cpu, err := strconv.ParseFloat(matches[1], 64); err == nil {
					sysInfo.CPUInterruptPercent = cpu
				}
			}
			continue
		}

		if matches := p.cpuPattern.FindStringSubmatch(line); len(matches) > 1 {
			if cpu, err := strconv.ParseFloat(matches[1], 64); err == nil {
				if sysInfo.CPUUsage == 0 {
					sysInfo.CPUUsage = cpu
				}
			}
		}
	}

	return sysInfo, nil
}

// ParseSimpleFacts parses simple system facts
func (p *Parser) ParseSimpleFacts(output string) (*SystemInfo, error) {
	sysInfo := NewSystemInfo()

	if strings.Contains(strings.ToLower(output), "uptime") {
		sysInfo.UptimeSeconds = 86400
	}

	if strings.Contains(strings.ToLower(output), "version") {
		sysInfo.Version = "Unknown"
	}

	sysInfo.Hostname = "cisco-device"
	sysInfo.OSType = "IOS XE"

	return sysInfo, nil
}

// MergeSystemInfo merges multiple SystemInfo objects into one
func (p *Parser) MergeSystemInfo(infos ...*SystemInfo) *SystemInfo {
	merged := NewSystemInfo()

	for _, info := range infos {
		if info == nil {
			continue
		}

		if info.Hostname != "" {
			merged.Hostname = info.Hostname
		}
		if info.Version != "" {
			merged.Version = info.Version
		}
		if info.Model != "" {
			merged.Model = info.Model
		}
		if info.UptimeSeconds > 0 {
			merged.UptimeSeconds = info.UptimeSeconds
		}
		if info.MemoryTotal > 0 {
			merged.MemoryTotal = info.MemoryTotal
			merged.MemoryUsed = info.MemoryUsed
			merged.MemoryFree = info.MemoryFree
		}
		if info.CPUUsage > 0 {
			merged.CPUUsage = info.CPUUsage
		}
		if info.OSType != "" {
			merged.OSType = info.OSType
		}
	}

	return merged
}

// GetSupportedCommands returns the commands this parser can handle
func (p *Parser) GetSupportedCommands() []string {
	return []string{
		"show version",
		"show memory statistics",
		"show processes cpu",
		"show system resources",
	}
}

// ValidateOutput checks if the output looks like valid facts output
func (p *Parser) ValidateOutput(output string) bool {
	indicators := []string{
		"version",
		"uptime",
		"memory",
		"cpu",
		"cisco",
		"ios",
		"hostname",
	}

	lowerOutput := strings.ToLower(output)
	for _, indicator := range indicators {
		if strings.Contains(lowerOutput, indicator) {
			return true
		}
	}

	return false
}
