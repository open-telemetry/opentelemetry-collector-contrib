// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !linux && !darwin && !freebsd && !openbsd
// +build !linux,!darwin,!freebsd,!openbsd

package processscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/processscraper"

const enableProcessesCount = false
const enableProcessesCreated = false

func (s *scraper) getAggregateProcessMetadata(_ processHandles) (aggregateProcessMetadata, error) {
	return aggregateProcessMetadata{}, nil
}
