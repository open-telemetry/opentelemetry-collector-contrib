// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build aix

package datadogextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/datadogextension"
import "go.opentelemetry.io/collector/extension"

func NewFactory() extension.Factory {
	panic("aix is not supported")
}
