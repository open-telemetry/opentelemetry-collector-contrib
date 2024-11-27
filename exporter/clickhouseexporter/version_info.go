// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package clickhouseexporter

import (
	"runtime"
	"runtime/debug"
)

type CollectorVersionResolver interface {
	// GetVersion returns the collector build information for use in query tracking.
	// Version should not include any slashes.
	GetVersion() string
}

// BinaryCollectorVersionResolver will use the Go binary to detect the collector version.
type BinaryCollectorVersionResolver struct {
	version string
}

func NewBinaryCollectorVersionResolver() *BinaryCollectorVersionResolver {
	resolver := BinaryCollectorVersionResolver{}

	osInformation := runtime.GOOS[:3] + "-" + runtime.GOARCH
	resolver.version = "unknown-" + osInformation

	info, ok := debug.ReadBuildInfo()
	if ok && info.Main.Version != "" {
		resolver.version = info.Main.Version + "-" + osInformation
	}

	return &resolver
}

func (r *BinaryCollectorVersionResolver) GetVersion() string {
	return r.version
}
