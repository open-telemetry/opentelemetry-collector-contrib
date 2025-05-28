// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package nfsscraper

import (
	"fmt"
	"strconv"
)

// parseStringsToUint64s parses a slice of strings into a slice of uint64
func parseStringsToUint64s(strSlice []string) (*[]uint64, error) {
	uint64Slice := make([]uint64, 0, len(strSlice))

	for _, s := range strSlice {
		val, err := strconv.ParseUint(s, 10, 64) // Assuming decimal base and uint64
		if err != nil {
			return nil, fmt.Errorf("failed to convert string '%s' to uint64: %w", s, err)
		}
		uint64Slice = append(uint64Slice, val)
	}

	return &uint64Slice, nil
}

func nothing(_ ...any) {
}
