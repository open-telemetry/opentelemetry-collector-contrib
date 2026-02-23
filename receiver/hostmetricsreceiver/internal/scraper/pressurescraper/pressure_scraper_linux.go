// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package pressurescraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/pressurescraper"

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/scraper/scrapererror"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/pressurescraper/internal/metadata"
)

const (
	pressureProcPath = "/proc/pressure"
	metricsLen       = 4
)

type pressureResource string

const (
	cpuResource pressureResource = "cpu"
	memResource pressureResource = "memory"
	ioResource  pressureResource = "io"
	irqResource pressureResource = "irq"
)

func (s *pressureScraper) recordPressureMetrics(now pcommon.Timestamp) error {
	var errors scrapererror.ScrapeErrors
	pressureDir := filepath.Join(s.config.rootPath, pressureProcPath)

	cpuStats, err := getPressureStats(filepath.Join(pressureDir, string(cpuResource)))
	if err != nil {
		errors.AddPartial(metricsLen, err)
	} else if cpuStats != nil {
		s.recordCPUPressure(now, cpuStats)
	}

	memStats, err := getPressureStats(filepath.Join(pressureDir, string(memResource)))
	if err != nil {
		errors.AddPartial(metricsLen, err)
	} else if memStats != nil {
		s.recordMemoryPressure(now, memStats)
	}

	ioStats, err := getPressureStats(filepath.Join(pressureDir, string(ioResource)))
	if err != nil {
		errors.AddPartial(metricsLen, err)
	} else if ioStats != nil {
		s.recordIOPressure(now, ioStats)
	}

	irqStats, err := getPressureStats(filepath.Join(pressureDir, string(irqResource)))
	if err != nil {
		errors.AddPartial(metricsLen, err)
	} else if irqStats != nil {
		s.recordIRQPressure(now, irqStats)
	}

	return errors.Combine()
}

func (s *pressureScraper) recordCPUPressure(now pcommon.Timestamp, stats *PressureResourceStats) {
	s.mb.RecordSystemCPULinuxPressure10sDataPoint(now, stats.some.avg10, metadata.AttributeSystemPressureStallTypeSome)
	s.mb.RecordSystemCPULinuxPressure1mDataPoint(now, stats.some.avg60, metadata.AttributeSystemPressureStallTypeSome)
	s.mb.RecordSystemCPULinuxPressure5mDataPoint(now, stats.some.avg300, metadata.AttributeSystemPressureStallTypeSome)
	s.mb.RecordSystemCPULinuxPressureTotalDataPoint(now, int64(stats.some.total), metadata.AttributeSystemPressureStallTypeSome)

	s.mb.RecordSystemCPULinuxPressure10sDataPoint(now, stats.full.avg10, metadata.AttributeSystemPressureStallTypeFull)
	s.mb.RecordSystemCPULinuxPressure1mDataPoint(now, stats.full.avg60, metadata.AttributeSystemPressureStallTypeFull)
	s.mb.RecordSystemCPULinuxPressure5mDataPoint(now, stats.full.avg300, metadata.AttributeSystemPressureStallTypeFull)
	s.mb.RecordSystemCPULinuxPressureTotalDataPoint(now, int64(stats.full.total), metadata.AttributeSystemPressureStallTypeFull)
}

func (s *pressureScraper) recordMemoryPressure(now pcommon.Timestamp, stats *PressureResourceStats) {
	s.mb.RecordSystemMemoryLinuxPressure10sDataPoint(now, stats.some.avg10, metadata.AttributeSystemPressureStallTypeSome)
	s.mb.RecordSystemMemoryLinuxPressure1mDataPoint(now, stats.some.avg60, metadata.AttributeSystemPressureStallTypeSome)
	s.mb.RecordSystemMemoryLinuxPressure5mDataPoint(now, stats.some.avg300, metadata.AttributeSystemPressureStallTypeSome)
	s.mb.RecordSystemMemoryLinuxPressureTotalDataPoint(now, int64(stats.some.total), metadata.AttributeSystemPressureStallTypeSome)

	s.mb.RecordSystemMemoryLinuxPressure10sDataPoint(now, stats.full.avg10, metadata.AttributeSystemPressureStallTypeFull)
	s.mb.RecordSystemMemoryLinuxPressure1mDataPoint(now, stats.full.avg60, metadata.AttributeSystemPressureStallTypeFull)
	s.mb.RecordSystemMemoryLinuxPressure5mDataPoint(now, stats.full.avg300, metadata.AttributeSystemPressureStallTypeFull)
	s.mb.RecordSystemMemoryLinuxPressureTotalDataPoint(now, int64(stats.full.total), metadata.AttributeSystemPressureStallTypeFull)
}

func (s *pressureScraper) recordIOPressure(now pcommon.Timestamp, stats *PressureResourceStats) {
	s.mb.RecordSystemIoLinuxPressure10sDataPoint(now, stats.some.avg10, metadata.AttributeSystemPressureStallTypeSome)
	s.mb.RecordSystemIoLinuxPressure1mDataPoint(now, stats.some.avg60, metadata.AttributeSystemPressureStallTypeSome)
	s.mb.RecordSystemIoLinuxPressure5mDataPoint(now, stats.some.avg300, metadata.AttributeSystemPressureStallTypeSome)
	s.mb.RecordSystemIoLinuxPressureTotalDataPoint(now, int64(stats.some.total), metadata.AttributeSystemPressureStallTypeSome)

	s.mb.RecordSystemIoLinuxPressure10sDataPoint(now, stats.full.avg10, metadata.AttributeSystemPressureStallTypeFull)
	s.mb.RecordSystemIoLinuxPressure1mDataPoint(now, stats.full.avg60, metadata.AttributeSystemPressureStallTypeFull)
	s.mb.RecordSystemIoLinuxPressure5mDataPoint(now, stats.full.avg300, metadata.AttributeSystemPressureStallTypeFull)
	s.mb.RecordSystemIoLinuxPressureTotalDataPoint(now, int64(stats.full.total), metadata.AttributeSystemPressureStallTypeFull)
}

func (s *pressureScraper) recordIRQPressure(now pcommon.Timestamp, stats *PressureResourceStats) {
	s.mb.RecordSystemIrqLinuxPressure10sDataPoint(now, stats.some.avg10, metadata.AttributeSystemPressureStallTypeSome)
	s.mb.RecordSystemIrqLinuxPressure1mDataPoint(now, stats.some.avg60, metadata.AttributeSystemPressureStallTypeSome)
	s.mb.RecordSystemIrqLinuxPressure5mDataPoint(now, stats.some.avg300, metadata.AttributeSystemPressureStallTypeSome)
	s.mb.RecordSystemIrqLinuxPressureTotalDataPoint(now, int64(stats.some.total), metadata.AttributeSystemPressureStallTypeSome)

	s.mb.RecordSystemIrqLinuxPressure10sDataPoint(now, stats.full.avg10, metadata.AttributeSystemPressureStallTypeFull)
	s.mb.RecordSystemIrqLinuxPressure1mDataPoint(now, stats.full.avg60, metadata.AttributeSystemPressureStallTypeFull)
	s.mb.RecordSystemIrqLinuxPressure5mDataPoint(now, stats.full.avg300, metadata.AttributeSystemPressureStallTypeFull)
	s.mb.RecordSystemIrqLinuxPressureTotalDataPoint(now, int64(stats.full.total), metadata.AttributeSystemPressureStallTypeFull)
}

func getPressureStats(pressureFile string) (*PressureResourceStats, error) {
	f, err := os.Open(pressureFile)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	return parsePressureStats(f)
}

func parsePressureStats(f io.Reader) (*PressureResourceStats, error) {
	resourceStats := &PressureResourceStats{}

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)

		if len(fields) < 5 {
			return nil, fmt.Errorf("invalid line (<5 fields): %v", line)
		}

		var stats PressureStats

		for _, field := range fields[1:] {
			key, val, found := strings.Cut(field, "=")
			if !found {
				continue
			}

			var err error
			switch key {
			case "avg10":
				stats.avg10, err = strconv.ParseFloat(val, 64)
				if err != nil {
					return nil, err
				}
			case "avg60":
				stats.avg60, err = strconv.ParseFloat(val, 64)
				if err != nil {
					return nil, err
				}
			case "avg300":
				stats.avg300, err = strconv.ParseFloat(val, 64)
				if err != nil {
					return nil, err
				}
			case "total":
				stats.total, err = strconv.ParseUint(val, 10, 64)
				if err != nil {
					return nil, err
				}
			}
		}

		switch fields[0] {
		case "some":
			resourceStats.some = stats
		case "full":
			resourceStats.full = stats
		default:
			continue
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error scanning Linux pressure stats: %w", err)
	}

	return resourceStats, nil
}
