// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build debug

package nfsscraper

import (
	"fmt"
	"os"
)

func debugLine(prefix, line string) {
	fmt.Fprintf(os.Stderr, "%s: %s\n", prefix, line)
}
