// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package optics // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ciscoosreceiver/internal/collectors/optics"

import (
	"errors"
	"regexp"
	"strings"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ciscoosreceiver/internal/rpc"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ciscoosreceiver/internal/util"
)

// Parser handles parsing of optical transceiver data from Cisco devices
type Parser struct{}

// NewParser creates a new Parser instance
func NewParser() *Parser {
	return &Parser{}
}

// ParseTransceivers parses optical transceiver information from command output
func (p *Parser) ParseTransceivers(output string) ([]*Transceiver, error) {
	var transceivers []*Transceiver

	lines := strings.Split(output, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		transceiver := p.parseTransceiverLine(line)
		if transceiver != nil {
			transceivers = append(transceivers, transceiver)
		}
	}

	return transceivers, nil
}

// parseTransceiverLine attempts to parse a single line for transceiver data
func (p *Parser) parseTransceiverLine(line string) *Transceiver {
	if len(line) > 5000 {
		line = line[:5000]
	}

	if strings.TrimSpace(line) == "" {
		return nil
	}

	patterns := []*regexp.Regexp{
		regexp.MustCompile(`^([a-zA-Z0-9\/\.-]+)\s+([\-\d\.]+)\s+([\-\d\.]+)`),
		regexp.MustCompile(`^([a-zA-Z0-9\/\.-]+)\s+Tx:\s*([\-\d\.]+)\s*dBm\s+Rx:\s*([\-\d\.]+)\s*dBm`),
	}

	for _, pattern := range patterns {
		matches := pattern.FindStringSubmatch(line)
		if len(matches) >= 4 {
			interfaceName := matches[1]
			txPower := util.Str2float64(matches[2])
			rxPower := util.Str2float64(matches[3])

			transceiver := NewTransceiver(interfaceName)
			transceiver.SetPowerLevels(txPower, rxPower)
			return transceiver
		}
	}

	return nil
}

// ParseOpticsData parses optics data for a specific interface
func (p *Parser) ParseOpticsData(osType rpc.OSType, output string) (*Transceiver, error) {
	switch osType {
	case rpc.IOS:
		return p.parseIOSOptics(output)
	case rpc.IOSXE:
		return p.parseIOSXEOptics(output)
	case rpc.NXOS:
		return p.parseNXOSOptics(output)
	default:
		return nil, errors.New("unsupported OS type for optics parsing")
	}
}

// parseIOSOptics parses IOS optics output
func (p *Parser) parseIOSOptics(output string) (*Transceiver, error) {
	pattern := regexp.MustCompile(`\S+\s+[\d\.]+\s+[\d\.]+\s+[\d\.]+\s+([\-\d\.]+)\s+([\-\d\.]+)`)
	matches := pattern.FindStringSubmatch(output)

	if len(matches) < 3 {
		return nil, errors.New("no optics data found in IOS output")
	}

	transceiver := NewTransceiver("")
	transceiver.SetPowerLevels(
		util.Str2float64(matches[1]),
		util.Str2float64(matches[2]),
	)

	return transceiver, nil
}

// parseIOSXEOptics parses IOS-XE optics output
func (p *Parser) parseIOSXEOptics(output string) (*Transceiver, error) {
	txPattern := regexp.MustCompile(`Transceiver Tx power\s*=\s*([\-\d\.]+)\s*dBm`)
	rxPattern := regexp.MustCompile(`Transceiver Rx optical power\s*=\s*([\-\d\.]+)\s*dBm`)

	txMatches := txPattern.FindStringSubmatch(output)
	rxMatches := rxPattern.FindStringSubmatch(output)

	if len(txMatches) < 2 || len(rxMatches) < 2 {
		return nil, errors.New("no optics data found in IOS-XE output")
	}

	transceiver := NewTransceiver("")
	transceiver.SetPowerLevels(
		util.Str2float64(txMatches[1]),
		util.Str2float64(rxMatches[1]),
	)

	return transceiver, nil
}

// parseNXOSOptics parses NX-OS optics output
func (p *Parser) parseNXOSOptics(output string) (*Transceiver, error) {
	txPattern := regexp.MustCompile(`Tx Power\s+([\-\d\.]+)\s*dBm`)
	rxPattern := regexp.MustCompile(`Rx Power\s+([\-\d\.]+)\s*dBm`)

	txMatches := txPattern.FindStringSubmatch(output)
	rxMatches := rxPattern.FindStringSubmatch(output)

	if len(txMatches) < 2 || len(rxMatches) < 2 {
		return nil, errors.New("no optics data found in NX-OS output")
	}

	transceiver := NewTransceiver("")
	transceiver.SetPowerLevels(
		util.Str2float64(txMatches[1]),
		util.Str2float64(rxMatches[1]),
	)

	return transceiver, nil
}
