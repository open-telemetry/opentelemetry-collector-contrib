// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package psiscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/psiscraper"

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/scraper"
	"go.opentelemetry.io/collector/scraper/scrapererror"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/psiscraper/internal/metadata"
)

const (
	cpuFile    = "/proc/pressure/cpu"
	memoryFile = "/proc/pressure/memory"
	ioFile     = "/proc/pressure/io"
)

// psiStats represents parsed PSI statistics for a resource
type psiStats struct {
	some psiStat
	full psiStat
}

type psiStat struct {
	avg10  float64
	avg60  float64
	avg300 float64
	total  int64
}

type psiScraper struct {
	settings scraper.Settings
	config   *Config
	mb       *metadata.MetricsBuilder
}

func newPSIScraper(_ context.Context, settings scraper.Settings, cfg *Config) *psiScraper {
	return &psiScraper{settings: settings, config: cfg}
}

func (s *psiScraper) start(_ context.Context, _ component.Host) error {
	s.mb = metadata.NewMetricsBuilder(s.config.MetricsBuilderConfig, s.settings)
	return nil
}

func (s *psiScraper) scrape(_ context.Context) (pmetric.Metrics, error) {
	now := pcommon.NewTimestampFromTime(time.Now())
	var errors scrapererror.ScrapeErrors

	// Get root path
	rootPath := s.config.RootPath
	if rootPath == "" {
		rootPath = "/"
	}

	cpuStats, err := readPSIFile(filepath.Join(rootPath, cpuFile))
	if err != nil {
		errors.AddPartial(4, err)
	} else {
		s.mb.RecordSystemPsiCPUSomeAvg10DataPoint(now, cpuStats.some.avg10)
		s.mb.RecordSystemPsiCPUSomeAvg60DataPoint(now, cpuStats.some.avg60)
		s.mb.RecordSystemPsiCPUSomeAvg300DataPoint(now, cpuStats.some.avg300)
		s.mb.RecordSystemPsiCPUSomeTotalDataPoint(now, cpuStats.some.total)
	}

	memStats, err := readPSIFile(filepath.Join(rootPath, memoryFile))
	if err != nil {
		errors.AddPartial(8, err)
	} else {
		s.mb.RecordSystemPsiMemorySomeAvg10DataPoint(now, memStats.some.avg10)
		s.mb.RecordSystemPsiMemorySomeAvg60DataPoint(now, memStats.some.avg60)
		s.mb.RecordSystemPsiMemorySomeAvg300DataPoint(now, memStats.some.avg300)
		s.mb.RecordSystemPsiMemorySomeTotalDataPoint(now, memStats.some.total)
		s.mb.RecordSystemPsiMemoryFullAvg10DataPoint(now, memStats.full.avg10)
		s.mb.RecordSystemPsiMemoryFullAvg60DataPoint(now, memStats.full.avg60)
		s.mb.RecordSystemPsiMemoryFullAvg300DataPoint(now, memStats.full.avg300)
		s.mb.RecordSystemPsiMemoryFullTotalDataPoint(now, memStats.full.total)
	}

	ioStats, err := readPSIFile(filepath.Join(rootPath, ioFile))
	if err != nil {
		errors.AddPartial(8, err)
	} else {
		s.mb.RecordSystemPsiIoSomeAvg10DataPoint(now, ioStats.some.avg10)
		s.mb.RecordSystemPsiIoSomeAvg60DataPoint(now, ioStats.some.avg60)
		s.mb.RecordSystemPsiIoSomeAvg300DataPoint(now, ioStats.some.avg300)
		s.mb.RecordSystemPsiIoSomeTotalDataPoint(now, ioStats.some.total)
		s.mb.RecordSystemPsiIoFullAvg10DataPoint(now, ioStats.full.avg10)
		s.mb.RecordSystemPsiIoFullAvg60DataPoint(now, ioStats.full.avg60)
		s.mb.RecordSystemPsiIoFullAvg300DataPoint(now, ioStats.full.avg300)
		s.mb.RecordSystemPsiIoFullTotalDataPoint(now, ioStats.full.total)
	}

	return s.mb.Emit(), errors.Combine()
}

func readPSIFile(path string) (*psiStats, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read PSI file %s: %w", path, err)
	}

	return parsePSI(string(content))
}

// parsePSI parses PSI file content
// Format:
// some avg10=0.00 avg60=0.00 avg300=0.00 total=0
// full avg10=0.00 avg60=0.00 avg300=0.00 total=0
func parsePSI(content string) (*psiStats, error) {
	stats := &psiStats{}
	lines := strings.Split(strings.TrimSpace(content), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) < 5 {
			return nil, fmt.Errorf("invalid PSI line format: %s", line)
		}

		pressureType := parts[0]
		var avg10, avg60, avg300 float64
		var total int64
		var err error

		// Parse avg10=X.XX
		if strings.HasPrefix(parts[1], "avg10=") {
			avg10, err = strconv.ParseFloat(strings.TrimPrefix(parts[1], "avg10="), 64)
			if err != nil {
				return nil, fmt.Errorf("failed to parse avg10: %w", err)
			}
		}

		// Parse avg60=X.XX
		if strings.HasPrefix(parts[2], "avg60=") {
			avg60, err = strconv.ParseFloat(strings.TrimPrefix(parts[2], "avg60="), 64)
			if err != nil {
				return nil, fmt.Errorf("failed to parse avg60: %w", err)
			}
		}

		// Parse avg300=X.XX
		if strings.HasPrefix(parts[3], "avg300=") {
			avg300, err = strconv.ParseFloat(strings.TrimPrefix(parts[3], "avg300="), 64)
			if err != nil {
				return nil, fmt.Errorf("failed to parse avg300: %w", err)
			}
		}

		// Parse total=XXXXX
		if strings.HasPrefix(parts[4], "total=") {
			total, err = strconv.ParseInt(strings.TrimPrefix(parts[4], "total="), 10, 64)
			if err != nil {
				return nil, fmt.Errorf("failed to parse total: %w", err)
			}
		}

		switch pressureType {
		case "some":
			stats.some.avg10 = avg10
			stats.some.avg60 = avg60
			stats.some.avg300 = avg300
			stats.some.total = total
		case "full":
			stats.full.avg10 = avg10
			stats.full.avg60 = avg60
			stats.full.avg300 = avg300
			stats.full.total = total
		default:
			return nil, fmt.Errorf("unknown pressure type: %s", pressureType)
		}
	}

	return stats, nil
}
