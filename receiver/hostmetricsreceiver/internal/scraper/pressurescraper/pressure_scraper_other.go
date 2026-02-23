// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !linux

package pressurescraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/pressurescraper"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func (s *pressureScraper) recordPressureMetrics(_ pcommon.Timestamp) error {
	return nil
}
