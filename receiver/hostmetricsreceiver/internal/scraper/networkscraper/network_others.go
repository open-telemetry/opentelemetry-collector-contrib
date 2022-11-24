// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !linux
// +build !linux

package networkscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/networkscraper"

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/networkscraper/bcal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/networkscraper/internal/metadata"
	"go.opentelemetry.io/collector/pdata/pcommon"
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

func (s *scraper) recordNetworkConntrackMetrics() error {
	return nil
}

func (s *scraper) recordSystemNetworkIoBandwidth(now pcommon.Timestamp, networkBandwidth bcal.NetworkBandwidth) {
	if s.config.Metrics.SystemNetworkIoBandwidth.Enabled {
		s.mb.RecordSystemNetworkIoBandwidthDataPoint(now, networkBandwidth.InboundRate, metadata.AttributeDirectionReceive)
		s.mb.RecordSystemNetworkIoBandwidthDataPoint(now, networkBandwidth.OutboundRate, metadata.AttributeDirectionTransmit)
	}
}
