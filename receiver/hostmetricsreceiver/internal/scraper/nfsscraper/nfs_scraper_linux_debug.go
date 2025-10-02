// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build debug

package nfsscraper

import (
	"bufio"
	"fmt"
	"os"
)

func debugDump(prefix, path string) {
	f, err := os.Open(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: failed to open %s for debug dump: %v\n", prefix, path, err)
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fmt.Fprintf(os.Stderr, "%s: %s\n", prefix, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "%s: error scanning debug file %s: %v\n", prefix, path, err)
	}
}
