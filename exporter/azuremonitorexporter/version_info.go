// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azuremonitorexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azuremonitorexporter"

import (
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
)

var (
	once          sync.Once
	cachedVersion string
)

func getCollectorVersion() string {
	once.Do(func() {
		osInformation := runtime.GOOS[:3] + "-" + runtime.GOARCH
		unknownVersion := "otelc-unknown-" + osInformation

		info, ok := debug.ReadBuildInfo()
		if !ok {
			cachedVersion = unknownVersion
			return
		}

		for _, mod := range info.Deps {
			if mod.Path == "go.opentelemetry.io/collector" {
				// Extract the semantic version without metadata.
				semVer := strings.SplitN(mod.Version, "-", 2)[0]
				cachedVersion = "otelc-" + semVer + "-" + osInformation
				return
			}
		}

		cachedVersion = unknownVersion
	})

	return cachedVersion
}
