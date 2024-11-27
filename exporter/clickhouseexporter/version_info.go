// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package clickhouseexporter

import (
	"runtime"
	"runtime/debug"
)

type collectorVersionResolver interface {
	// GetVersion returns the collector build information for use in query tracking.
	// Version should not include any slashes.
	GetVersion() string
}

// binaryCollectorVersionResolver will use the Go binary to detect the collector version.
type binaryCollectorVersionResolver struct {
	version string
}

func newBinaryCollectorVersionResolver() *binaryCollectorVersionResolver {
	resolver := binaryCollectorVersionResolver{}

	osInformation := runtime.GOOS[:3] + "-" + runtime.GOARCH
	resolver.version = "unknown-" + osInformation

	info, ok := debug.ReadBuildInfo()
	if ok && info.Main.Version != "" {
		resolver.version = info.Main.Version + "-" + osInformation
	}

	return &resolver
}

func (r *binaryCollectorVersionResolver) GetVersion() string {
	return r.version
}
