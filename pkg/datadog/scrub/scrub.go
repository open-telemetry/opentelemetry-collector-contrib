// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package scrub contains a Scrubber that scrubs error from sensitive details
package scrub // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog/scrub"

import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog/internal/scrub"

type Scrubber = scrub.Scrubber

func NewScrubber() Scrubber {
	return scrub.NewScrubber()
}
