// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package perfcounters // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/perfcounters"

import (
	"strings"
)

type PerfCounterInitError struct {
	FailedObjects []string
}

func (p *PerfCounterInitError) Error() string {
	return "failed to init counters: " + strings.Join(p.FailedObjects, "; ")
}

func (p *PerfCounterInitError) AddFailure(object string) {
	p.FailedObjects = append(p.FailedObjects, object)
}
