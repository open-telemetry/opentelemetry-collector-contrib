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
	"strings"
	"time"

	"github.com/shirou/gopsutil/v4/mem"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.uber.org/zap"
)

// recordSystemSpecificMetrics is called from scrape() in memory_scraper.go.
// Darwin implementation records:
//
// - system.darwin.memory.pressure
// - system.darwin.memory.compressor.pages
//
// Failures are non-fatal: log debug and continue.
func (s *memoryScraper) recordSystemSpecificMetrics(now pcommon.Timestamp, _ *mem.VirtualMemoryStat) {
	// 1) macOS memory pressure (sysctl kern.memorystatus_vm_pressure_level)
	pressure, err := readMemorystatusVMPressureLevel(context.Background())
	if err != nil {
		s.settings.Logger.Debug("failed to read macOS memory pressure level",
			zap.Error(err),
		)
	} else {
		// Generated from: system.darwin.memory.pressure
		s.mb.RecordSystemDarwinMemoryPressureDataPoint(now, pressure)
	}
	// 2) Pages occupied by compressor (vm_stat)
	pages, err := readVMStatCompressorPages(context.Background())
	if err != nil {
		s.settings.Logger.Debug("failed to read macOS compressor pages occupied",
			zap.Error(err),
		)
	} else {
		// Generated from: system.darwin.memory.compressor.pages
		s.mb.RecordSystemDarwinMemoryCompressorPagesDataPoint(now, pages)
	}
}

func readMemorystatusVMPressureLevel(ctx context.Context) (int64, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	out, err := exec.CommandContext(ctx, "/usr/sbin/sysctl", "-n", "kern.memorystatus_vm_pressure_level").Output()
	if err != nil {
		return 0, err
	}

	s := strings.TrimSpace(string(out))
	if s == "" {
		return 0, errors.New("empty sysctl output for kern.memorystatus_vm_pressure_level")
	}

	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse int from %q: %w", s, err)
	}
	return v, nil
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
