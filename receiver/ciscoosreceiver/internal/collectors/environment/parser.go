// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package environment // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ciscoosreceiver/internal/collectors/environment"

import (
	"regexp"
	"strconv"
	"strings"
)

// Parser handles parsing of environment command output
type Parser struct {
	temperaturePattern *regexp.Regexp
	powerSupplyPattern *regexp.Regexp
	fanPattern         *regexp.Regexp
}

// NewParser creates a new environment parser
func NewParser() *Parser {
	tempPattern := regexp.MustCompile(`(?i)temp.*?(\w+)\s+(\d+)\s+celsius\s+(\w+)`)
	psPattern := regexp.MustCompile(`(?i)power\s+supply\s+(\d+)\s+(\w+)`)
	fanPattern := regexp.MustCompile(`(?i)fan\s+(\d+)\s+(\d+)\s+rpm\s+(\w+)`)

	return &Parser{
		temperaturePattern: tempPattern,
		powerSupplyPattern: psPattern,
		fanPattern:         fanPattern,
	}
}

// ParseEnvironment parses the output of "show environment" command
func (p *Parser) ParseEnvironment(output string) ([]*Sensor, error) {
	var sensors []*Sensor

	lines := strings.Split(output, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if tempSensors := p.parseTemperatureLine(line); len(tempSensors) > 0 {
			sensors = append(sensors, tempSensors...)
		}

		if psSensors := p.parsePowerSupplyLine(line); len(psSensors) > 0 {
			sensors = append(sensors, psSensors...)
		}

		if fanSensors := p.parseFanLine(line); len(fanSensors) > 0 {
			sensors = append(sensors, fanSensors...)
		}
	}

	return sensors, nil
}

// parseTemperatureLine parses a line for temperature information
func (p *Parser) parseTemperatureLine(line string) []*Sensor {
	var sensors []*Sensor

	matches := p.temperaturePattern.FindAllStringSubmatch(line, -1)
	for _, match := range matches {
		if len(match) != 4 {
			continue
		}

		location := match[1]
		tempStr := match[2]
		status := match[3]

		temperature, err := strconv.ParseFloat(tempStr, 64)
		if err != nil {
			continue
		}

		sensor := NewTemperatureSensor("Temperature", location, temperature)
		sensor.SetStatus(status)

		if sensor.Validate() {
			sensors = append(sensors, sensor)
		}
	}

	return sensors
}

// parsePowerSupplyLine parses a line for power supply information
func (p *Parser) parsePowerSupplyLine(line string) []*Sensor {
	var sensors []*Sensor

	matches := p.powerSupplyPattern.FindAllStringSubmatch(line, -1)
	for _, match := range matches {
		if len(match) != 3 {
			continue
		}

		psNumber := match[1]
		status := match[2]

		name := "PowerSupply" + psNumber
		sensor := NewPowerSupplySensor(name, status)

		if sensor.Validate() {
			sensors = append(sensors, sensor)
		}
	}

	return sensors
}

// parseFanLine parses a line for fan information
func (p *Parser) parseFanLine(line string) []*Sensor {
	var sensors []*Sensor

	matches := p.fanPattern.FindAllStringSubmatch(line, -1)
	for _, match := range matches {
		if len(match) != 4 {
			continue
		}

		fanNumber := match[1]
		rpmStr := match[2]
		status := match[3]

		rpm, err := strconv.ParseFloat(rpmStr, 64)
		if err != nil {
			continue
		}

		name := "Fan" + fanNumber
		sensor := NewFanSensor(name, "Chassis", rpm)
		sensor.SetStatus(status)

		if sensor.Validate() {
			sensors = append(sensors, sensor)
		}
	}

	return sensors
}

// ParseSimpleTemperature parses simple temperature output
func (p *Parser) ParseSimpleTemperature(output string) ([]*Sensor, error) {
	var sensors []*Sensor

	simplePattern := regexp.MustCompile(`(?i)(?:temperature.*?)?(\d+)(?:c|celsius)?`)

	matches := simplePattern.FindAllStringSubmatch(output, -1)
	for i, match := range matches {
		if len(match) < 2 {
			continue
		}

		tempStr := match[1]
		temperature, err := strconv.ParseFloat(tempStr, 64)
		if err != nil {
			continue
		}

		name := "Temperature"
		if i > 0 {
			name = name + strconv.Itoa(i+1)
		}

		sensor := NewTemperatureSensor(name, "System", temperature)
		sensor.SetStatus("OK")

		if sensor.Validate() {
			sensors = append(sensors, sensor)
		}
	}

	return sensors, nil
}

// GetSupportedCommands returns the commands this parser can handle
func (p *Parser) GetSupportedCommands() []string {
	return []string{
		"show environment",
		"show environment temperature",
		"show environment power",
		"show environment fan",
	}
}

// ValidateOutput checks if the output looks like valid environment output
func (p *Parser) ValidateOutput(output string) bool {
	// Check for specific environment-related patterns
	environmentPatterns := []string{
		"temp:",
		"temperature",
		"power supply",
		"fan",
		"environment",
		"sensor",
		"celsius",
		"rpm",
	}

	lowerOutput := strings.ToLower(output)

	// Exclude interface-specific output
	if strings.Contains(lowerOutput, "interface") && strings.Contains(lowerOutput, "gigabitethernet") {
		return false
	}

	// Exclude generic text without environmental indicators
	if strings.Contains(lowerOutput, "random output") || strings.Contains(lowerOutput, "plain text") {
		return false
	}

	for _, pattern := range environmentPatterns {
		if strings.Contains(lowerOutput, pattern) {
			return true
		}
	}

	return false
}
