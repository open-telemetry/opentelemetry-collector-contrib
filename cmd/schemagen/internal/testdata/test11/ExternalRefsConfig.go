// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package test11

import "github.com/open-telemetry/opentelemetry-collector-contrib/cmd/schemagen/internal/testdata/external"

type ExternalRefsConfig struct {
	external.TestConfig `mapstructure:",squash"`
	Test                external.TestConfig `mapstructure:"test_nested"`
}
