// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !linux
// +build !linux

// Taken from https://github.com/signalfx/golib/blob/master/metadata/hostmetadata/host-not-linux.go as is.

package hostmetadata // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter/internal/hostmetadata"

func fillPlatformSpecificOSData(info *hostOS) error {
	return nil
}

func fillPlatformSpecificCPUData(info *hostCPU) error {
	return nil
}
