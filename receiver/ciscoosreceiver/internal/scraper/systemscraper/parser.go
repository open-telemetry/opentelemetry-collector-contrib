// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package systemscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ciscoosreceiver/internal/scraper/systemscraper"

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// parseCPUUtilizationNXOS parses "show system resources" output for NX-OS devices
// Example line: "CPU states  :   27.70% user,   10.80% kernel,   61.49% idle"
// Returns CPU utilization as fraction (0-1)
func parseCPUUtilizationNXOS(output string) (float64, error) {
	// Regex to match: CPU states  :   27.70% user,   10.80% kernel,   61.49% idle
	cpuRegex := regexp.MustCompile(`CPU states\s*:\s*([\d.]+)% user,\s*([\d.]+)% kernel`)

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		matches := cpuRegex.FindStringSubmatch(line)
		if len(matches) == 3 {
			userPercent, err1 := strconv.ParseFloat(matches[1], 64)
			kernelPercent, err2 := strconv.ParseFloat(matches[2], 64)

			if err1 != nil || err2 != nil {
				continue
			}

			// CPU utilization = user + kernel (as fraction)
			cpuUtilization := (userPercent + kernelPercent) / 100.0
			return cpuUtilization, nil
		}
	}

	return 0, fmt.Errorf("failed to parse CPU utilization from NX-OS output: %s", output)
}

// parseCPUUtilizationIOS parses "show process cpu" output for IOS/IOS XE devices
// Example line: "CPU utilization for five seconds: 8%/2%; one minute: 7%; five minutes: 6%"
// Returns CPU utilization as fraction (0-1) using 5-second average
func parseCPUUtilizationIOS(output string) (float64, error) {
	// Regex to match: CPU utilization for five seconds: 8%/2%; one minute: 7%; five minutes: 6%
	cpuRegex := regexp.MustCompile(`CPU utilization for five seconds:\s*(\d+)%`)

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		matches := cpuRegex.FindStringSubmatch(line)
		if len(matches) == 2 {
			fiveSecPercent, err := strconv.ParseFloat(matches[1], 64)
			if err != nil {
				continue
			}

			// CPU utilization as fraction
			cpuUtilization := fiveSecPercent / 100.0
			return cpuUtilization, nil
		}
	}

	return 0, fmt.Errorf("failed to parse CPU utilization from IOS output: %s", output)
}

// parseMemoryUtilizationNXOS parses "show system resources" output for NX-OS devices
// Example line: "Memory usage:   32803148K total,   11174924K used,   21628224K free"
// Returns memory utilization as fraction (0-1)
func parseMemoryUtilizationNXOS(output string) (float64, error) {
	// Regex to match: Memory usage:   32803148K total,   11174924K used,   21628224K free
	memRegex := regexp.MustCompile(`Memory usage:\s*(\d+)K total,\s*(\d+)K used`)

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		matches := memRegex.FindStringSubmatch(line)
		if len(matches) == 3 {
			totalKB, err1 := strconv.ParseFloat(matches[1], 64)
			usedKB, err2 := strconv.ParseFloat(matches[2], 64)

			if err1 != nil || err2 != nil {
				continue
			}

			if totalKB == 0 {
				return 0, fmt.Errorf("total memory is zero in NX-OS output: %s", output)
			}

			// Memory utilization as fraction
			memUtilization := usedKB / totalKB
			return memUtilization, nil
		}
	}

	return 0, fmt.Errorf("failed to parse memory utilization from NX-OS output: %s", output)
}

// parseMemoryUtilizationIOS parses "show process memory" output for IOS/IOS XE devices
// Example line: "Processor Pool Total:    1000000 Used:     600000 Free:     400000"
// Returns memory utilization as fraction (0-1)
func parseMemoryUtilizationIOS(output string) (float64, error) {
	// Regex to match: Processor Pool Total:    1000000 Used:     600000 Free:     400000
	// Flexible whitespace matching
	memRegex := regexp.MustCompile(`Processor\s+Pool\s+Total:\s*(\d+)\s+Used:\s*(\d+)`)

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		matches := memRegex.FindStringSubmatch(line)
		if len(matches) == 3 {
			total, err1 := strconv.ParseFloat(matches[1], 64)
			used, err2 := strconv.ParseFloat(matches[2], 64)

			if err1 != nil || err2 != nil {
				continue
			}

			if total == 0 {
				return 0, fmt.Errorf("total memory is zero in IOS output: %s", output)
			}

			// Memory utilization as fraction
			memUtilization := used / total
			return memUtilization, nil
		}
	}

	return 0, fmt.Errorf("failed to parse memory utilization from IOS output: %s", output)
}
