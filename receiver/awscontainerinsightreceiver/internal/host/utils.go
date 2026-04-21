// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package host // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/host"

import (
	"context"
	"hash/fnv"
	"os"
	"time"
)

const (
	rootfs     = "/rootfs"            // the root directory "/" is mounted as "/rootfs" in container
	hostProc   = rootfs + "/proc"     // "/rootfs/proc" in container refers to the host proc directory "/proc"
	hostMounts = hostProc + "/mounts" // "/rootfs/proc/mounts" in container refers to "/proc/mounts" in the host
)

func hostJitter(maxDuration time.Duration) time.Duration {
	hostName, err := os.Hostname()
	if err != nil {
		hostName = "Unknown"
	}
	hash := fnv.New64()
	hash.Write([]byte(hostName))
	// Right shift the uint64 hash by one to make sure the jitter duration is always positive
	hostSleepJitter := time.Duration(int64(hash.Sum64()>>1)) % maxDuration
	return hostSleepJitter
}

// RefreshUntil executes the refresh() function periodically with the given refresh interval
// until shouldRefresh() return false or the context is canceled
func RefreshUntil(ctx context.Context, refresh func(context.Context), refreshInterval time.Duration,
	shouldRefresh func() bool, maxJitterTime time.Duration,
) {
	if maxJitterTime > 0 {
		// add some sleep jitter to prevent a large number of receivers calling the ec2 api at the same time
		time.Sleep(hostJitter(maxJitterTime))
	}

	// initial refresh
	refresh(ctx)

	refreshTicker := time.NewTicker(refreshInterval)
	defer refreshTicker.Stop()
	for {
		select {
		case <-refreshTicker.C:
			if !shouldRefresh() {
				return
			}
			refresh(ctx)
		case <-ctx.Done():
			return
		}
	}
}
