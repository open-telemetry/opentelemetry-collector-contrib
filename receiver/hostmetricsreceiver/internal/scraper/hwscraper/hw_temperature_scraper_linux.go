// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package hwscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/hwscraper"

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/scraper/scrapererror"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterset"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/hwscraper/internal/metadata"
)

const (
	minReasonableTemp = -40.0
	maxReasonableTemp = 200.0
)

type hwTemperatureScraper struct {
	logger               *zap.Logger
	config               *TemperatureConfig
	hwmonPath            string
	metricsBuilderConfig metadata.MetricsBuilderConfig

	includeFilter filterset.FilterSet
	excludeFilter filterset.FilterSet

	sensors []sensorInfo
}

func (s *hwTemperatureScraper) start(_ context.Context) error {
	var err error

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

	if _, statErr := os.Stat(s.hwmonPath); os.IsNotExist(statErr) {
		s.logger.Error("hwmon path does not exist", zap.String("path", s.hwmonPath))
		return ErrHWMonUnavailable
	}

	s.sensors, err = s.scanTemperatureSensors()
	if err != nil {
		s.logger.Debug("Failed to scan temperature sensors", zap.Error(err))
	}

	return nil
}

func (s *hwTemperatureScraper) scrape(_ context.Context, mb *metadata.MetricsBuilder) error {
	now := pcommon.NewTimestampFromTime(time.Now())
	var errors scrapererror.ScrapeErrors

	if s.metricsBuilderConfig.Metrics.HwTemperature.Enabled {
		for _, sensor := range s.sensors {
			tempCelsius, err := s.readTemperatureCelsius(sensor.tempFile)
			if err != nil {
				errors.AddPartial(hwTemperatureMetricsLen, fmt.Errorf("failed to read temperature for %s: %w", sensor.label, err))
				continue
			}
			mb.RecordHwTemperatureDataPoint(now, tempCelsius, sensor.id, sensor.label, sensor.deviceName, sensor.location)
		}
	}

	if s.metricsBuilderConfig.Metrics.HwTemperatureLimit.Enabled {
		for _, sensor := range s.sensors {
			limits := s.readTemperatureLimits(sensor)
			s.recordTemperatureLimits(now, sensor, limits, mb)
		}
	}

	if s.metricsBuilderConfig.Metrics.HwStatus.Enabled {
		for _, sensor := range s.sensors {
			tempCelsius, err := s.readTemperatureCelsius(sensor.tempFile)
			if err != nil {
				errors.AddPartial(hwTemperatureMetricsLen, fmt.Errorf("failed to read temperature for %s: %w", sensor.label, err))
				continue
			}
			limits := s.readTemperatureLimits(sensor)
			s.recordStatusDataPoints(now, sensor, tempCelsius, limits, mb)
		}
	}

	return errors.Combine()
}

type temperatureLimits struct {
	maxTemp     *float64
	critTemp    *float64
	minTemp     *float64
	lowCritTemp *float64
}

type sensorInfo struct {
	id         string
	label      string
	deviceName string
	location   string
	hwmonDir   string
	sensorNum  string
	tempFile   string
}

func (s *hwTemperatureScraper) scanTemperatureSensors() ([]sensorInfo, error) {
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

func (*hwTemperatureScraper) readTemperatureCelsius(file string) (float64, error) {
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

func (s *hwTemperatureScraper) buildSensorInfo(tempFile, deviceName string) (sensorInfo, error) {
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

	sensor := sensorInfo{
		id:         fmt.Sprintf("%s_temp%s", deviceName, sensorNum),
		label:      sensorLabel,
		deviceName: deviceName,
		location:   fmt.Sprintf("%s_TEMP%s", strings.ToUpper(deviceName), sensorNum), // OpenTelemetry standard format
		hwmonDir:   filepath.Dir(tempFile),
		sensorNum:  sensorNum,
		tempFile:   tempFile,
	}

	return sensor, nil
}

func (s *hwTemperatureScraper) readTemperatureLimits(sensor sensorInfo) temperatureLimits {
	limits := temperatureLimits{}

	limitFiles := map[string]struct {
		limitType string
		target    **float64
	}{
		fmt.Sprintf("temp%s_crit", sensor.sensorNum):  {"high.critical", &limits.critTemp},
		fmt.Sprintf("temp%s_max", sensor.sensorNum):   {"high.degraded", &limits.maxTemp},
		fmt.Sprintf("temp%s_lcrit", sensor.sensorNum): {"low.critical", &limits.lowCritTemp},
		fmt.Sprintf("temp%s_min", sensor.sensorNum):   {"low.degraded", &limits.minTemp},
	}

	for limitFile, info := range limitFiles {
		limitPath := filepath.Join(sensor.hwmonDir, limitFile)
		if limitCelsius, err := s.readTemperatureCelsius(limitPath); err == nil {
			// Validate temperature limit is within reasonable range
			if limitCelsius >= minReasonableTemp && limitCelsius <= maxReasonableTemp {
				*info.target = &limitCelsius
			}
		}
	}

	return limits
}

func (*hwTemperatureScraper) recordTemperatureLimits(now pcommon.Timestamp, sensor sensorInfo, limits temperatureLimits, mb *metadata.MetricsBuilder) {
	limitFiles := map[string]struct {
		limitType string
		target    *float64
	}{
		fmt.Sprintf("temp%s_crit", sensor.sensorNum):  {"high.critical", limits.critTemp},
		fmt.Sprintf("temp%s_max", sensor.sensorNum):   {"high.degraded", limits.maxTemp},
		fmt.Sprintf("temp%s_lcrit", sensor.sensorNum): {"low.critical", limits.lowCritTemp},
		fmt.Sprintf("temp%s_min", sensor.sensorNum):   {"low.degraded", limits.minTemp},
	}

	for _, info := range limitFiles {
		if info.target != nil {
			mb.RecordHwTemperatureLimitDataPoint(
				now,
				*info.target,
				sensor.id,
				metadata.MapAttributeLimitType[info.limitType],
				sensor.label,
				sensor.deviceName,
				sensor.location,
			)
		}
	}
}

func (s *hwTemperatureScraper) recordStatusDataPoints(now pcommon.Timestamp, sensor sensorInfo, tempCelsius float64, limits temperatureLimits, mb *metadata.MetricsBuilder) {
	currentState := s.determineTemperatureState(tempCelsius, limits)

	allStates := []metadata.AttributeState{
		metadata.AttributeStateOk,
		metadata.AttributeStateDegraded,
		metadata.AttributeStateFailed,
		metadata.AttributeStateNeedsCleaning,
		metadata.AttributeStatePredictedFailure,
	}

	for _, state := range allStates {
		value := int64(0)
		if state == currentState {
			value = int64(1)
		}

		mb.RecordHwStatusDataPoint(
			now,
			value,
			sensor.id,
			state,
			metadata.AttributeTypeTemperature,
			sensor.label,
			sensor.deviceName,
		)
	}
}

func (s *hwTemperatureScraper) determineTemperatureState(currentTemp float64, limits temperatureLimits) metadata.AttributeState {
	if currentTemp < minReasonableTemp || currentTemp > maxReasonableTemp {
		return metadata.AttributeStateFailed
	}

	if s.predictFailure(currentTemp, limits) {
		return metadata.AttributeStatePredictedFailure
	}

	if s.isDegraded(currentTemp, limits) {
		return metadata.AttributeStateDegraded
	}

	if s.needsCleaning(currentTemp, limits) {
		return metadata.AttributeStateNeedsCleaning
	}

	return metadata.AttributeStateOk
}

func (*hwTemperatureScraper) predictFailure(currentTemp float64, limits temperatureLimits) bool {
	if limits.critTemp != nil && currentTemp >= *limits.critTemp {
		return true
	}
	if limits.lowCritTemp != nil && currentTemp <= *limits.lowCritTemp {
		return true
	}

	// Use default thresholds when sensor limits are not available
	if currentTemp > 85.0 {
		return true
	}
	if currentTemp < 0.0 {
		return true
	}

	return false
}

func (*hwTemperatureScraper) isDegraded(currentTemp float64, limits temperatureLimits) bool {
	if limits.maxTemp != nil && currentTemp >= *limits.maxTemp {
		return true
	}
	if limits.minTemp != nil && currentTemp <= *limits.minTemp {
		return true
	}

	// Use default thresholds when sensor limits are not available
	if currentTemp >= 80.0 {
		return true
	}
	if currentTemp <= 5.0 {
		return true
	}

	return false
}

func (*hwTemperatureScraper) needsCleaning(currentTemp float64, limits temperatureLimits) bool {
	if limits.maxTemp != nil {
		cleaningThreshold := *limits.maxTemp * 0.8
		if currentTemp >= cleaningThreshold {
			return true
		}
	}

	if currentTemp > 75.0 {
		return true
	}

	return false
}

func (*hwTemperatureScraper) getDeviceName(hwmonDir string) string {
	nameFile := filepath.Join(hwmonDir, "name")
	nameBytes, err := os.ReadFile(nameFile)
	if err != nil {
		return filepath.Base(hwmonDir) // fallback to directory name
	}
	return strings.TrimSpace(string(nameBytes))
}

func (*hwTemperatureScraper) getSensorLabel(labelFile, defaultLabel string) string {
	if labelBytes, err := os.ReadFile(labelFile); err == nil {
		return strings.TrimSpace(string(labelBytes))
	}
	return defaultLabel
}

func (s *hwTemperatureScraper) shouldIncludeSensor(sensorName string) bool {
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
