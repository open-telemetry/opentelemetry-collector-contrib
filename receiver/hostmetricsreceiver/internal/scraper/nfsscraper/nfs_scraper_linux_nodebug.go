// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build linux && !debug

package nfsscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/nfsscraper"

func debugLine(_, _ string) {
	// This is a no-op function that will be compiled in when the 'debug' build tag is not used.
	// go compiler will inline / optimize this call out
}
