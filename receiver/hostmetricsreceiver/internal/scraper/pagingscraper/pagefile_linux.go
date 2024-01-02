// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build linux
// +build linux

package pagingscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/pagingscraper"

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

const swapsFilePath = "/proc/swaps"

// swaps file column indexes
const (
	nameCol = 0
	// typeCol     = 1
	totalCol = 2
	usedCol  = 3
	// priorityCol = 4

	minimumColCount = usedCol + 1
)

func getPageFileStats() ([]*pageFileStats, error) {
	f, err := os.Open(swapsFilePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	return parseSwapsFile(f)
}

func parseSwapsFile(r io.Reader) ([]*pageFileStats, error) {
	scanner := bufio.NewScanner(r)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return nil, fmt.Errorf("couldn't read file %q: %w", swapsFilePath, err)
		}
		return nil, fmt.Errorf("unexpected end-of-file in %q", swapsFilePath)

	}

	// Check header headerFields are as expected
	headerFields := strings.Fields(scanner.Text())
	if len(headerFields) < minimumColCount {
		return nil, fmt.Errorf("couldn't parse %q: expected ≥%d fields in header but got %v", swapsFilePath, minimumColCount, headerFields)
	}
	if headerFields[nameCol] != "Filename" {
		return nil, fmt.Errorf("couldn't parse %q: expected %q to be %q", swapsFilePath, headerFields[nameCol], "Filename")
	}
	if headerFields[totalCol] != "Size" {
		return nil, fmt.Errorf("couldn't parse %q: expected %q to be %q", swapsFilePath, headerFields[totalCol], "Size")
	}
	if headerFields[usedCol] != "Used" {
		return nil, fmt.Errorf("couldn't parse %q: expected %q to be %q", swapsFilePath, headerFields[usedCol], "Used")
	}

	var swapDevices []*pageFileStats
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < minimumColCount {
			return nil, fmt.Errorf("couldn't parse %q: expected ≥%d fields in row but got %v", swapsFilePath, minimumColCount, fields)
		}

		totalKiB, err := strconv.ParseUint(fields[totalCol], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("couldn't parse 'Size' column in %q: %w", swapsFilePath, err)
		}

		usedKiB, err := strconv.ParseUint(fields[usedCol], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("couldn't parse 'Used' column in %q: %w", swapsFilePath, err)
		}

		swapDevices = append(swapDevices, &pageFileStats{
			deviceName: fields[nameCol],
			usedBytes:  usedKiB * 1024,
			freeBytes:  (totalKiB - usedKiB) * 1024,
			totalBytes: totalKiB * 1024,
		})
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("couldn't read file %q: %w", swapsFilePath, err)
	}

	return swapDevices, nil
}
