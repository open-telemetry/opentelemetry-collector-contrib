// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// +build freebsd

package pagingscraper

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

const swapCommand = "/sbin/swapctl"

// swapctl column indexes
const (
	nameCol     = 0
	totalKiBCol = 1
	usedKiBCol  = 2
)

func getPageFileStats() ([]*pageFileStats, error) {
	output, err := exec.Command(swapCommand, "-lk").Output()
	if err != nil {
		return nil, fmt.Errorf("could not execute %q: %w", swapCommand, err)
	}

	return parseSwapctlOutput(string(output))
}

func parseSwapctlOutput(output string) ([]*pageFileStats, error) {
	lines := strings.Split(output, "\n")
	if len(lines) == 0 {
		return nil, fmt.Errorf("could not parse output of %q: no lines in %q", swapCommand, output)
	}

	// Check header headerFields are as expected.
	header := lines[0]
	header = strings.ToLower(header)
	header = strings.ReplaceAll(header, ":", "")
	headerFields := strings.Fields(header)
	if len(headerFields) < usedKiBCol {
		return nil, fmt.Errorf("couldn't parse %q: too few fields in header %q", swapCommand, header)
	}
	if headerFields[nameCol] != "device" {
		return nil, fmt.Errorf("couldn't parse %q: expected %q to be %q", swapCommand, headerFields[nameCol], "device")
	}
	if headerFields[totalKiBCol] != "1kb-blocks" && headerFields[totalKiBCol] != "1k-blocks" {
		return nil, fmt.Errorf("couldn't parse %q: expected %q to be %q", swapCommand, headerFields[totalKiBCol], "1kb-blocks")
	}
	if headerFields[usedKiBCol] != "used" {
		return nil, fmt.Errorf("couldn't parse %q: expected %q to be %q", swapCommand, headerFields[usedKiBCol], "used")
	}

	var swapDevices []*pageFileStats
	for _, line := range lines[1:] {
		if line == "" {
			continue // the terminal line is typically empty
		}
		fields := strings.Fields(line)
		if len(fields) < usedKiBCol {
			return nil, fmt.Errorf("couldn't parse %q: too few fields", swapCommand)
		}

		totalKiB, err := strconv.ParseUint(fields[totalKiBCol], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("couldn't parse 'Size' column in %q: %w", swapCommand, err)
		}

		usedKiB, err := strconv.ParseUint(fields[usedKiBCol], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("couldn't parse 'Used' column in %q: %w", swapCommand, err)
		}

		swapDevices = append(swapDevices, &pageFileStats{
			deviceName: fields[nameCol],
			usedBytes:  usedKiB * 1024,
			freeBytes:  (totalKiB - usedKiB) * 1024,
		})
	}

	return swapDevices, nil
}
