// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package nfsscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/nfsscraper"

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
)

const (
	nfsProcFile = "/proc/net/rpc/nfs"
	nfsdProcFile = "/proc/net/rpc/nfsd"
)

func getNfsStats() (*NfsStats, error) {
	f, err := os.Open(nfsProcFile)
        if err != nil {
                return nil, err
        }
        defer f.Close()

	return parseNfsStats(f)
}

func parseNfsStats(f io.Reader) (*NfsStats, error) {
	nfsStats := &NfsStats{}

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)

		if len(fields) < 2 {
			return nil, fmt.Errorf("Invalid line (<2 fields) in %v: %v", nfsProcFile, line)
		}

		values, err := ParseStringsToUint64s(fields[1:])
		if err != nil {
			return nil, fmt.Errorf("error parsing line in %v: %v: %w", nfsProcFile, line, err)
		}

		nothing(values)
	}

	nothing(nfsStats)
	return nil, nil
}

func getNfsdStats() (*NfsdStats, error) {
	return nil, nil
}
