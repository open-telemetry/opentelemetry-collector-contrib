// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !linux && !darwin && !freebsd && !openbsd

package processesscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/processesscraper"

import (
	"context"
)

const (
	enableProcessesCount   = false
	enableProcessesCreated = false
)

func (s *processesScraper) getProcessesMetadata(context.Context) (processesMetadata, error) {
	return processesMetadata{}, nil
}
