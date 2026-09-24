// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package networkscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/networkscraper"

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/shirou/gopsutil/v4/common"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/gopsutilenv"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/networkscraper/ucal"
)

const (
	iffLoopback = 0x8
	unknownU16  = 65535
	unknownU32  = 4294967295
)

func readNetworkLinkInfo(ctx context.Context, device string) (ucal.LinkInfo, error) {
	devicePath := gopsutilenv.GetEnvWithContext(ctx, string(common.HostSysEnvKey), "/sys", "class", "net", device)
	speedMbps := readSpeedMbps(filepath.Join(devicePath, "speed"))
	return ucal.LinkInfo{
		SpeedMbps:  speedMbps,
		FullDuplex: readDuplex(filepath.Join(devicePath, "duplex")),
		Loopback:   readLoopback(filepath.Join(devicePath, "flags")),
	}, nil
}

func readSpeedMbps(path string) int64 {
	contents, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	speed, err := strconv.ParseInt(strings.TrimSpace(string(contents)), 10, 64)
	if err != nil || speed < 0 || speed == unknownU16 || speed == unknownU32 {
		return 0
	}
	return speed
}

func readDuplex(path string) bool {
	contents, err := os.ReadFile(path)
	return err == nil && strings.EqualFold(strings.TrimSpace(string(contents)), "full")
}

func readLoopback(path string) bool {
	contents, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	flags, err := strconv.ParseUint(strings.TrimSpace(string(contents)), 0, 64)
	if err != nil {
		return false
	}
	return flags&iffLoopback != 0
}
