// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build aix

package pebbletailstorageextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/tailstorage/pebbletailstorageextension"

import "go.opentelemetry.io/collector/extension"

func NewFactory() extension.Factory {
	panic("aix is not supported")
}
