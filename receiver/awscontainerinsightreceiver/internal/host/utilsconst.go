// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !windows
// +build !windows

package host // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/host"

const (
	rootfs     = "/rootfs"            // the root directory "/" is mounted as "/rootfs" in container
	hostProc   = rootfs + "/proc"     // "/rootfs/proc" in container refers to the host proc directory "/proc"
	hostMounts = hostProc + "/mounts" // "/rootfs/proc/mounts" in container refers to "/proc/mounts" in the host
)
