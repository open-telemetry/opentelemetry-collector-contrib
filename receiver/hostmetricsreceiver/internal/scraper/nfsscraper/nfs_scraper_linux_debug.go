// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build linux && debug

package nfsscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/nfsscraper"

import (
	"fmt"
	"os"
)

func debugLine(prefix, line string) {
	fmt.Fprintf(os.Stderr, "%s: %s\n", prefix, line)
}
