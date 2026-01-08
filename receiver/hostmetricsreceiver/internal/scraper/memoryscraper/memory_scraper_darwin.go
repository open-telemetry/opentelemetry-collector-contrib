// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build darwin

package memoryscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/memoryscraper"
import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"

	"github.com/shirou/gopsutil/v4/mem"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.uber.org/zap"
	"golang.org/x/sys/unix"
)

// recordSystemSpecificMetrics is called from scrape() in memory_scraper.go.
func (s *memoryScraper) recordSystemSpecificMetrics(now pcommon.Timestamp, _ *mem.VirtualMemoryStat) {
	pressure, err := readMemorystatusVMPressureLevel(context.Background())
	if err != nil {
		s.settings.Logger.Debug("failed to read macOS memory pressue level",
			zap.Error(err),
		)
	} else {
		s.mb.RecordSystemMemoryDarwinPressureStatusDataPoint(now, pressure)
	}
	pages, err := readVMStatCompressorPages(context.Background())
	if err != nil {
		s.settings.Logger.Debug("failed to read macOS compressor pages occupied",
			zap.Error(err),
		)
	} else {
		s.mb.RecordSystemMemoryDarwinCompressorPagesDataPoint(now, pages)
	}
}

func readMemorystatusVMPressureLevel(_ context.Context) (int64, error) {
	val, err := unix.SysctlUint32("kern.memorystatus_vm_pressure_level")
	if err != nil {
		return 0, fmt.Errorf("failed to read sysctl kern.memorystatus_vm_pressure_level: %w", err)
	}

	return int64(val), nil
}

// vm_stat line example:
// Pages occupied by compressor:        12345.
var reVMStatCompressor = regexp.MustCompile(`(?m)^Pages occupied by compressor:\s+(\d+)\.`)

func readVMStatCompressorPages(ctx context.Context) (int64, error) {
	cmd := exec.CommandContext(ctx, "/usr/bin/vm_stat")
	out, err := cmd.Output()
	if err != nil {
		return 0, err
	}
	m := reVMStatCompressor.FindSubmatch(out)
	if len(m) < 2 {
		return 0, errors.New("could not find 'Pages occupied by compressor' in vm_stat output")
	}
	raw := bytes.TrimSpace(m[1])
	v, err := strconv.ParseInt(string(raw), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse int from %q: %w", string(raw), err)
	}
	return v, nil
}
