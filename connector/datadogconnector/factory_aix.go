// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build aix

package datadogconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/datadogconnector"
import "go.opentelemetry.io/collector/connector"

func NewFactory() connector.Factory {
	panic("aix is not supported")
}
