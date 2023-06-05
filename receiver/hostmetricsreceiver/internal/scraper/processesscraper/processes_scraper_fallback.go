// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !linux && !darwin && !freebsd && !openbsd
// +build !linux,!darwin,!freebsd,!openbsd

package processesscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/processesscraper"

const enableProcessesCount = false
const enableProcessesCreated = false

func (s *scraper) getProcessesMetadata() (processesMetadata, error) {
	return processesMetadata{}, nil
}
