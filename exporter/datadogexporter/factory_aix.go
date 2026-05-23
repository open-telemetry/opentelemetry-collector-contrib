// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !aix

package datadogexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter"

import "go.opentelemetry.io/collector/exporter"

func NewFactory() exporter.Factory {
	panic("aix is not supported")
}
