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

// +build solaris

package pagingscraper

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

const swapsCommand = "swap"

// The blockSize as reported by `swap -l`. See https://docs.oracle.com/cd/E23824_01/html/821-1459/fsswap-52195.html
const blockSize = 512

// swapctl column indexes
const (
	nameCol = 0
	// devCol = 1
	// swaploCol = 2
	totalBlocksCol = 3
	freeBlocksCol  = 4
)

func getPageFileStats() ([]*pageFileStats, error) {
	output, err := exec.Command(swapsCommand, "-l").Output()
	if err != nil {
		return nil, fmt.Errorf("could not execute %q: %w", swapsCommand, err)
	}

	return parseSwapsCommandOutput(string(output))
}

func parseSwapsCommandOutput(output string) ([]*pageFileStats, error) {
	lines := strings.Split(output, "\n")
	if len(lines) == 0 {
		return nil, fmt.Errorf("could not parse output of %q: no lines in %q", swapsCommand, output)
	}

	// Check header headerFields are as expected.
	headerFields := strings.Fields(lines[0])
	if len(headerFields) < freeBlocksCol {
		return nil, fmt.Errorf("couldn't parse %q: too few fields in header %q", swapsCommand, lines[0])
	}
	if headerFields[nameCol] != "swapfile" {
		return nil, fmt.Errorf("couldn't parse %q: expected %q to be %q", swapsCommand, headerFields[nameCol], "swapfile")
	}
	if headerFields[totalBlocksCol] != "blocks" {
		return nil, fmt.Errorf("couldn't parse %q: expected %q to be %q", swapsCommand, headerFields[totalBlocksCol], "blocks")
	}
	if headerFields[freeBlocksCol] != "free" {
		return nil, fmt.Errorf("couldn't parse %q: expected %q to be %q", swapsCommand, headerFields[freeBlocksCol], "free")
	}

	var swapDevices []*pageFileStats
	for _, line := range lines[1:] {
		if line == "" {
			continue // the terminal line is typically empty
		}
		fields := strings.Fields(line)
		if len(fields) < freeBlocksCol {
			return nil, fmt.Errorf("couldn't parse %q: too few fields", swapsCommand)
		}

		totalBlocks, err := strconv.ParseUint(fields[totalBlocksCol], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("couldn't parse 'Size' column in %q: %w", swapsCommand, err)
		}

		freeBlocks, err := strconv.ParseUint(fields[freeBlocksCol], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("couldn't parse 'Used' column in %q: %w", swapsCommand, err)
		}

		swapDevices = append(swapDevices, &pageFileStats{
			deviceName: fields[nameCol],
			usedBytes:  (totalBlocks - freeBlocks) * blockSize,
			freeBytes:  freeBlocks * blockSize,
		})
	}

	return swapDevices, nil
}
