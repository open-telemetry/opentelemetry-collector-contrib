//go:build linux

// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package edacscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/edacscraper"

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/scraper"
	"go.opentelemetry.io/collector/scraper/scrapererror"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/edacscraper/internal/metadata"
)

const (
	edacSysPath = "/sys/devices/system/edac/mc"
)

var (
	memControllerRE = regexp.MustCompile(`.*devices/system/edac/mc/mc([0-9]+)`)
	memCsrowRE      = regexp.MustCompile(`.*devices/system/edac/mc/mc[0-9]+/csrow([0-9]+)`)
)

// scraper for EDAC Metrics
type edacScraper struct {
	settings scraper.Settings
	config   *Config
	mb       *metadata.MetricsBuilder

	// for mocking file operations during testing
	readFromFile func(filename string) ([]byte, error)
	glob         func(pattern string) ([]string, error)
}

// newEDACMetricsScraper creates an EDAC Scraper
func newEDACMetricsScraper(_ context.Context, settings scraper.Settings, cfg *Config) *edacScraper {
	return &edacScraper{
		settings:     settings,
		config:       cfg,
		readFromFile: os.ReadFile,
		glob:         filepath.Glob,
	}
}

func (s *edacScraper) start(ctx context.Context, _ component.Host) error {
	s.mb = metadata.NewMetricsBuilder(s.config.MetricsBuilderConfig, s.settings, metadata.WithStartTime(pcommon.Timestamp(time.Now().UnixNano())))
	return nil
}

func (s *edacScraper) scrape(ctx context.Context) (pmetric.Metrics, error) {
	now := pcommon.NewTimestampFromTime(time.Now())

	// Find all memory controllers
	controllers, err := s.glob(edacSysPath + "/mc[0-9]*")
	if err != nil {
		return pmetric.NewMetrics(), fmt.Errorf("failed to find EDAC memory controllers: %w", err)
	}

	var scrapeErrors []error

	for _, controller := range controllers {
		controllerMatch := memControllerRE.FindStringSubmatch(controller)
		if controllerMatch == nil {
			scrapeErrors = append(scrapeErrors, fmt.Errorf("controller path didn't match regexp: %s", controller))
			continue
		}
		controllerID := controllerMatch[1]

		// Scrape controller-level correctable errors
		if err := s.scrapeControllerErrors(now, controller, controllerID, "ce_count", metadata.AttributeEdacErrorTypeCorrectable); err != nil {
			scrapeErrors = append(scrapeErrors, err)
		}

		// Scrape controller-level uncorrectable errors
		if err := s.scrapeControllerErrors(now, controller, controllerID, "ue_count", metadata.AttributeEdacErrorTypeUncorrectable); err != nil {
			scrapeErrors = append(scrapeErrors, err)
		}

		// Scrape controller-level ce_noinfo_count (goes to csrow metrics with "unknown" csrow)
		if err := s.scrapeCsrowErrors(now, controller, controllerID, "unknown", "ce_noinfo_count", metadata.AttributeEdacErrorTypeCorrectable); err != nil {
			scrapeErrors = append(scrapeErrors, err)
		}

		// Scrape controller-level ue_noinfo_count (goes to csrow metrics with "unknown" csrow)
		if err := s.scrapeCsrowErrors(now, controller, controllerID, "unknown", "ue_noinfo_count", metadata.AttributeEdacErrorTypeUncorrectable); err != nil {
			scrapeErrors = append(scrapeErrors, err)
		}

		// Scrape CSROW-level errors
		if scrapeErr := s.scrapeCsrows(now, controller, controllerID); scrapeErr != nil {
			scrapeErrors = append(scrapeErrors, scrapeErr)
		}
	}

	var scrapeErr error
	if len(scrapeErrors) > 0 {
		scrapeErr = scrapererror.NewPartialScrapeError(fmt.Errorf("errors occurred while scraping EDAC metrics: %v", scrapeErrors), len(scrapeErrors))
	}

	return s.mb.Emit(), scrapeErr
}

func (s *edacScraper) scrapeControllerErrors(now pcommon.Timestamp, controllerPath, controllerID, fileName string, errorType metadata.AttributeEdacErrorType) error {
	filePath := filepath.Join(controllerPath, fileName)
	value, err := s.readIntFromFile(filePath)
	if err != nil {
		return fmt.Errorf("couldn't read %s for controller %s: %w", fileName, controllerID, err)
	}

	s.mb.RecordSystemEdacMemoryErrorsDataPoint(now, value, controllerID, errorType)
	return nil
}

func (s *edacScraper) scrapeCsrowErrors(now pcommon.Timestamp, controllerPath, controllerID, csrowID, fileName string, errorType metadata.AttributeEdacErrorType) error {
	filePath := filepath.Join(controllerPath, fileName)
	value, err := s.readIntFromFile(filePath)
	if err != nil {
		return fmt.Errorf("couldn't read %s for controller %s csrow %s: %w", fileName, controllerID, csrowID, err)
	}

	s.mb.RecordSystemEdacMemoryCsrowErrorsDataPoint(now, value, controllerID, csrowID, errorType)
	return nil
}

func (s *edacScraper) scrapeCsrows(now pcommon.Timestamp, controllerPath, controllerID string) error {
	// Find all csrows for this controller
	csrows, err := s.glob(controllerPath + "/csrow[0-9]*")
	if err != nil {
		return fmt.Errorf("failed to find CSROW directories for controller %s: %w", controllerID, err)
	}

	for _, csrow := range csrows {
		csrowMatch := memCsrowRE.FindStringSubmatch(csrow)
		if csrowMatch == nil {
			return fmt.Errorf("csrow path didn't match regexp: %s", csrow)
		}
		csrowID := csrowMatch[1]

		// Scrape correctable errors for this csrow
		if err := s.scrapeCsrowFileErrors(now, csrow, controllerID, csrowID, "ce_count", metadata.AttributeEdacErrorTypeCorrectable); err != nil {
			return err
		}

		// Scrape uncorrectable errors for this csrow
		if err := s.scrapeCsrowFileErrors(now, csrow, controllerID, csrowID, "ue_count", metadata.AttributeEdacErrorTypeUncorrectable); err != nil {
			return err
		}
	}

	return nil
}

func (s *edacScraper) scrapeCsrowFileErrors(now pcommon.Timestamp, csrowPath, controllerID, csrowID, fileName string, errorType metadata.AttributeEdacErrorType) error {
	filePath := filepath.Join(csrowPath, fileName)
	value, err := s.readIntFromFile(filePath)
	if err != nil {
		return fmt.Errorf("couldn't read %s for controller %s csrow %s: %w", fileName, controllerID, csrowID, err)
	}

	s.mb.RecordSystemEdacMemoryCsrowErrorsDataPoint(now, value, controllerID, csrowID, errorType)
	return nil
}

func (s *edacScraper) readIntFromFile(filename string) (int64, error) {
	bytes, err := s.readFromFile(filename)
	if err != nil {
		return 0, err
	}

	str := strings.TrimSpace(string(bytes))
	value, err := strconv.ParseInt(str, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse integer from %s: %w", filename, err)
	}

	return value, nil
}
