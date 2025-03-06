// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !linux

package networkscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/networkscraper"
import (
	"context"
)

var allTCPStates = []string{
	"CLOSE_WAIT",
	"CLOSED",
	"CLOSING",
	"DELETE",
	"ESTABLISHED",
	"FIN_WAIT_1",
	"FIN_WAIT_2",
	"LAST_ACK",
	"LISTEN",
	"SYN_SENT",
	"SYN_RECEIVED",
	"TIME_WAIT",
}

func (s *networkScraper) recordNetworkConntrackMetrics(context.Context) error {
	return nil
}
