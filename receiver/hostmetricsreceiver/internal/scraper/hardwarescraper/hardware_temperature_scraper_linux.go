// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package hardwarescraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/hardwarescraper"

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v4/common"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/scraper/scrapererror"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterset"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/gopsutilenv"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/hardwarescraper/internal/metadata"
)

const (
	minReasonableTemp = -40.0
	maxReasonableTemp = 200.0
)

type hardwareTemperatureScraper struct {
	logger               *zap.Logger
	config               *TemperatureConfig
	hwmonPath            string
	metricsBuilderConfig metadata.MetricsBuilderConfig

	includeFilter filterset.FilterSet
	excludeFilter filterset.FilterSet
}

func (s *hardwareTemperatureScraper) start(ctx context.Context) error {
	var err error

	if s.hwmonPath == "" || s.hwmonPath == defaultHwmonPath {
		s.hwmonPath = gopsutilenv.GetEnvWithContext(ctx, string(common.HostSysEnvKey), "/sys", "class", "hwmon")
	}

	if len(s.config.Include.Sensors) > 0 {
		s.includeFilter, err = filterset.CreateFilterSet(s.config.Include.Sensors, &s.config.Include.Config)
		if err != nil {
			return fmt.Errorf("failed to create include filter: %w", err)
		}
	}

	if len(s.config.Exclude.Sensors) > 0 {
		s.excludeFilter, err = filterset.CreateFilterSet(s.config.Exclude.Sensors, &s.config.Exclude.Config)
		if err != nil {
			return fmt.Errorf("failed to create exclude filter: %w", err)
		}
	}

	return nil
}

func (s *hardwareTemperatureScraper) scrape(_ context.Context, mb *metadata.MetricsBuilder) error {
	now := pcommon.NewTimestampFromTime(time.Now())
	var errors scrapererror.ScrapeErrors

	// Sensors are enumerated on every scrape rather than cached at start, so that
	// hwmon devices appearing or disappearing at runtime (module load, hotplug) are
	// reflected in the emitted metrics.
	sensors, err := s.scanTemperatureSensors()
	if err != nil {
		errors.AddPartial(hardwareTemperatureMetricsLen, fmt.Errorf("failed to scan temperature sensors: %w", err))
		return errors.Combine()
	}

	if s.metricsBuilderConfig.Metrics.HwTemperature.Enabled {
		for _, sensor := range sensors {
			tempCelsius, err := s.readTemperatureCelsius(sensor.tempFile)
			if err != nil {
				errors.AddPartial(hardwareTemperatureMetricsLen, fmt.Errorf("failed to read temperature for %s: %w", sensor.location, err))
				continue
			}
			mb.RecordHwTemperatureDataPoint(now, tempCelsius, sensor.id, sensor.name, sensor.parent, sensor.location)
		}
	}

	if s.metricsBuilderConfig.Metrics.HwTemperatureLimit.Enabled {
		for _, sensor := range sensors {
			s.recordTemperatureLimits(now, sensor, mb)
		}
	}

	return errors.Combine()
}

// temperatureLimitTypes maps the hwmon threshold file suffix to the semconv
// limit type it represents.
var temperatureLimitTypes = []struct {
	suffix    string
	limitType metadata.AttributeHwLimitType
}{
	{"crit", metadata.AttributeHwLimitTypeHighCritical},
	{"max", metadata.AttributeHwLimitTypeHighDegraded},
	{"lcrit", metadata.AttributeHwLimitTypeLowCritical},
	{"min", metadata.AttributeHwLimitTypeLowDegraded},
}

type sensorInfo struct {
	id        string
	name      string
	parent    string
	location  string
	hwmonDir  string
	sensorNum string
	tempFile  string
}

func (s *hardwareTemperatureScraper) scanTemperatureSensors() ([]sensorInfo, error) {
	var sensors []sensorInfo

	hwmonDirs, err := filepath.Glob(filepath.Join(s.hwmonPath, "hwmon*"))
	if err != nil {
		return nil, fmt.Errorf("failed to find hwmon directories: %w", err)
	}

	for _, hwmonDir := range hwmonDirs {
		deviceName := s.getDeviceName(hwmonDir)

		tempFiles, err := filepath.Glob(filepath.Join(hwmonDir, "temp*_input"))
		if err != nil {
			s.logger.Debug("Failed to find temperature files", zap.String("dir", hwmonDir), zap.Error(err))
			continue
		}

		for _, tempFile := range tempFiles {
			sensor, err := s.buildSensorInfo(tempFile, deviceName)
			if err != nil {
				s.logger.Debug("Failed to build sensor info", zap.String("file", tempFile), zap.Error(err))
				continue
			}
			sensors = append(sensors, sensor)
		}
	}

	return sensors, nil
}

func (*hardwareTemperatureScraper) readTemperatureCelsius(file string) (float64, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return 0, err
	}

	valueStr := strings.TrimSpace(string(data))
	tempMilliCelsius, err := strconv.Atoi(valueStr)
	if err != nil {
		return 0, err
	}
	return float64(tempMilliCelsius) / 1000.0, nil
}

// deviceKey returns a stable per-device identifier for a hwmon directory so that
// distinct devices sharing the same name (e.g. multiple NVMe drives) do not
// collide. It resolves the underlying device path, which is stable across reboots
// (e.g. a PCI address), and falls back to the hwmon directory name when there is
// no device link (e.g. some virtual sensors).
func deviceKey(hwmonDir string) string {
	if target, err := filepath.EvalSymlinks(filepath.Join(hwmonDir, "device")); err == nil {
		return filepath.Base(target)
	}
	return filepath.Base(hwmonDir)
}

func (s *hardwareTemperatureScraper) buildSensorInfo(tempFile, deviceName string) (sensorInfo, error) {
	baseName := filepath.Base(tempFile)
	sensorNum := extractSensorNumber(baseName)
	if sensorNum == "" {
		return sensorInfo{}, fmt.Errorf("failed to extract sensor number from %s", baseName)
	}

	labelFile := filepath.Join(filepath.Dir(tempFile), fmt.Sprintf("temp%s_label", sensorNum))
	sensorLabel := s.getSensorLabel(labelFile, fmt.Sprintf("temp%s", sensorNum))

	if !s.shouldIncludeSensor(sensorLabel) {
		return sensorInfo{}, fmt.Errorf("sensor %s filtered out", sensorLabel)
	}

	hwmonDir := filepath.Dir(tempFile)
	deviceID := fmt.Sprintf("%s_%s", deviceName, deviceKey(hwmonDir))

	sensor := sensorInfo{
		id:        fmt.Sprintf("%s_temp%s", deviceID, sensorNum),
		name:      deviceName,
		parent:    deviceID,
		location:  sensorLabel,
		hwmonDir:  hwmonDir,
		sensorNum: sensorNum,
		tempFile:  tempFile,
	}

	return sensor, nil
}

func (s *hardwareTemperatureScraper) recordTemperatureLimits(now pcommon.Timestamp, sensor sensorInfo, mb *metadata.MetricsBuilder) {
	for _, limit := range temperatureLimitTypes {
		limitPath := filepath.Join(sensor.hwmonDir, fmt.Sprintf("temp%s_%s", sensor.sensorNum, limit.suffix))

		limitCelsius, err := s.readTemperatureCelsius(limitPath)
		if err != nil {
			continue
		}
		// Validate temperature limit is within reasonable range
		if limitCelsius < minReasonableTemp || limitCelsius > maxReasonableTemp {
			continue
		}

		mb.RecordHwTemperatureLimitDataPoint(
			now,
			limitCelsius,
			sensor.id,
			limit.limitType,
			sensor.name,
			sensor.parent,
			sensor.location,
		)
	}
}

func (*hardwareTemperatureScraper) getDeviceName(hwmonDir string) string {
	nameFile := filepath.Join(hwmonDir, "name")
	nameBytes, err := os.ReadFile(nameFile)
	if err != nil {
		return filepath.Base(hwmonDir) // fallback to directory name
	}
	return strings.TrimSpace(string(nameBytes))
}

func (*hardwareTemperatureScraper) getSensorLabel(labelFile, defaultLabel string) string {
	if labelBytes, err := os.ReadFile(labelFile); err == nil {
		return strings.TrimSpace(string(labelBytes))
	}
	return defaultLabel
}

func (s *hardwareTemperatureScraper) shouldIncludeSensor(sensorName string) bool {
	if s.excludeFilter != nil && s.excludeFilter.Matches(sensorName) {
		return false
	}

	if s.includeFilter != nil {
		return s.includeFilter.Matches(sensorName)
	}

	return true
}

func extractSensorNumber(filename string) string {
	re := regexp.MustCompile(`temp(\d+)_input`)
	matches := re.FindStringSubmatch(filename)
	if len(matches) >= 2 {
		return matches[1]
	}
	return ""
}
