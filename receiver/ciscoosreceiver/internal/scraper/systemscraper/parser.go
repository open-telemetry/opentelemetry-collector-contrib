// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package systemscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ciscoosreceiver/internal/scraper/systemscraper"

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// parseCPUUtilizationNXOS parses NX-OS CPU stats (fraction 0-1)
func parseCPUUtilizationNXOS(output string) (float64, error) {
	// Regex to match: CPU states  :   27.70% user,   10.80% kernel,   61.49% idle
	cpuRegex := regexp.MustCompile(`CPU states\s*:\s*([\d.]+)% user,\s*([\d.]+)% kernel`)

	lines := strings.SplitSeq(output, "\n")
	for line := range lines {
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

// parseCPUUtilizationIOS parses IOS CPU stats using 5-second average (fraction 0-1)
func parseCPUUtilizationIOS(output string) (float64, error) {
	// Regex to match: CPU utilization for five seconds: 8%/2%; one minute: 7%; five minutes: 6%
	cpuRegex := regexp.MustCompile(`CPU utilization for five seconds:\s*(\d+)%`)

	lines := strings.SplitSeq(output, "\n")
	for line := range lines {
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

// parseMemoryUtilization parses memory stats based on OS type (fraction 0-1)
func parseMemoryUtilization(output, osType string) (float64, error) {
	var memRegex *regexp.Regexp

	// Select regex pattern based on OS type
	if osType == "NX-OS" {
		// NX-OS: "Memory usage:   32803148K total,   11174924K used,   21628224K free"
		memRegex = regexp.MustCompile(`Memory usage:\s*(\d+)K total,\s*(\d+)K used`)
	} else {
		// IOS/IOS XE: "Processor Pool Total:    1000000 Used:     600000 Free:     400000"
		memRegex = regexp.MustCompile(`Processor\s+Pool\s+Total:\s*(\d+)\s+Used:\s*(\d+)`)
	}

	lines := strings.SplitSeq(output, "\n")
	for line := range lines {
		matches := memRegex.FindStringSubmatch(line)
		if len(matches) == 3 {
			total, err1 := strconv.ParseFloat(matches[1], 64)
			used, err2 := strconv.ParseFloat(matches[2], 64)

			if err1 != nil || err2 != nil {
				continue
			}

			if total == 0 {
				return 0, fmt.Errorf("total memory is zero in %s output: %s", osType, output)
			}

			// Memory utilization as fraction
			memUtilization := used / total
			return memUtilization, nil
		}
	}

	return 0, fmt.Errorf("failed to parse memory utilization from %s output: %s", osType, output)
}
