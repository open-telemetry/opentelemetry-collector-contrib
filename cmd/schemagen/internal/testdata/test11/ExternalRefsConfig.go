// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package test11

import (
	"go.opentelemetry.io/collector/scraper/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/schemagen/internal/testdata/external"
)

type ExternalRefsConfig struct {
	scraperhelper.ControllerConfig `mapstructure:",squash"`
	external.TestConfig            `mapstructure:",squash"`
	Test                           external.TestConfig `mapstructure:"test_nested"`
}
