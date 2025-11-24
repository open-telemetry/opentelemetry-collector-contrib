// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package interfacesscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ciscoosreceiver/internal/scraper/interfacesscraper"

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"go.uber.org/zap"
)

// Interface represents a network interface on a Cisco device
type Interface struct {
	Name        string
	MACAddress  string
	Description string

	OperStatus string

	InputErrors  float64
	OutputErrors float64

	InputDrops  float64
	OutputDrops float64

	InputBytes  float64
	OutputBytes float64

	InputBroadcast float64
	InputMulticast float64

	Speed       int64
	SpeedString string
}

const (
	StatusUp   = "up"
	StatusDown = "down"
)

func NewInterface(name string) *Interface {
	return &Interface{
		Name:       name,
		OperStatus: StatusDown,
	}
}

// GetOperStatusInt converts operational status to integer (1=up, 0=down)
func (i *Interface) GetOperStatusInt() int64 {
	if i.OperStatus == StatusUp {
		return 1
	}
	return 0
}

// Validate ensures interface has required data and valid status
func (i *Interface) Validate() bool {
	if i.Name == "" {
		return false
	}
	if i.OperStatus != StatusUp && i.OperStatus != StatusDown {
		i.OperStatus = StatusDown
	}
	return true
}

// parseStatus normalizes status strings to "up" or "down"
func parseStatus(status string) string {
	switch status {
	case "up", "UP", "Up", "1":
		return StatusUp
	case "down", "DOWN", "Down", "0":
		return StatusDown
	default:
		return StatusDown
	}
}

// formatSpeed converts speed in bps to human-readable format
func formatSpeed(speedBps int64) string {
	if speedBps <= 0 {
		return ""
	}

	switch {
	case speedBps >= 1000000000:
		return fmt.Sprintf("%.0f Gb/s", float64(speedBps)/1000000000)
	case speedBps >= 1000000:
		return fmt.Sprintf("%.0f Mb/s", float64(speedBps)/1000000)
	case speedBps >= 1000:
		return fmt.Sprintf("%.0f Kb/s", float64(speedBps)/1000)
	default:
		return fmt.Sprintf("%d b/s", speedBps)
	}
}

// str2float64 converts string to float64
func str2float64(s string) float64 {
	if s == "-" || s == "" {
		return 0
	}
	if val, err := strconv.ParseFloat(s, 64); err == nil {
		return val
	}
	return 0
}

// parseInterfaces parses interface information from command output
func parseInterfaces(output string, logger *zap.Logger) []*Interface {
	txNXOS := regexp.MustCompile(`^\s+TX$`)
	macRegexp := regexp.MustCompile(`^\s+Hardware(?: is|:) .+, address(?: is|:) ([0-9a-fA-F]{4}\.[0-9a-fA-F]{4}\.[0-9a-fA-F]{4})`)
	deviceNameRegexp := regexp.MustCompile(`^([a-zA-Z0-9/.-]+) is.*$`)
	adminStatusRegexp := regexp.MustCompile(`^.+ is (administratively)?\s*(up|down).*, line protocol is.*$`)
	adminStatusNXOSRegexp := regexp.MustCompile(`^\S+ is (up|down)(?:\s|,)?(\(Administratively down\))?.*$`)
	descRegexp := regexp.MustCompile(`^\s+Description: (.*)$`)
	dropsRegexp := regexp.MustCompile(`^\s+Input queue: \d+\/\d+\/(\d+|-)\/\d+ .+ Total output drops: (\d+|-)$`)
	multiBroadNXOS := regexp.MustCompile(`^.* (\d+|-) multicast packets?\s+(\d+|-) broadcast pack`)
	multiBroadIOSXE := regexp.MustCompile(`^\s+Received\s+(\d+|-)\sbroadcasts \((\d+|-) (?:IP\s)?multicasts?\)`)
	multiBroadIOS := regexp.MustCompile(`^\s+Received (\d+|-) broadcasts.*$`)
	inputBytesRegexp := regexp.MustCompile(`^\s+\d+ (?:packets input,|input packets)\s+(\d+|-) bytes.*$`)
	outputBytesRegexp := regexp.MustCompile(`^\s+\d+ (?:packets output,|output packets)\s+(\d+|-) bytes.*$`)
	inputErrorsRegexp := regexp.MustCompile(`^\s+(\d+|-) input error(?:s,)? .*$`)
	outputErrorsRegexp := regexp.MustCompile(`^\s+(\d+|-) output error(?:s,)? .*$`)
	speedRegexp := regexp.MustCompile(`^\s+(.*)-duplex,\s(\d+) ((\wb)/s).*$`)
	newIfRegexp := regexp.MustCompile(`(?:^!?(?: |admin|show|.+#).*$|^$)`)

	isRx := true
	var current *Interface
	var interfaces []*Interface
	lines := strings.SplitSeq(output, "\n")

	for line := range lines {
		if !newIfRegexp.MatchString(line) {
			if current != nil && current.Validate() {
				interfaces = append(interfaces, current)
			}
			matches := deviceNameRegexp.FindStringSubmatch(line)
			if matches == nil {
				continue
			}
			current = NewInterface(matches[1])
			isRx = true
		}
		if current == nil {
			continue
		}
		switch {
		case adminStatusRegexp.MatchString(line):
			matches := adminStatusRegexp.FindStringSubmatch(line)
			current.OperStatus = parseStatus(matches[2])
		case adminStatusNXOSRegexp.MatchString(line):
			matches := adminStatusNXOSRegexp.FindStringSubmatch(line)
			current.OperStatus = parseStatus(matches[1])
		case descRegexp.MatchString(line):
			matches := descRegexp.FindStringSubmatch(line)
			current.Description = matches[1]
		case macRegexp.MatchString(line):
			matches := macRegexp.FindStringSubmatch(line)
			current.MACAddress = matches[1]
		case dropsRegexp.MatchString(line):
			matches := dropsRegexp.FindStringSubmatch(line)
			current.InputDrops = str2float64(matches[1])
			current.OutputDrops = str2float64(matches[2])
		case inputBytesRegexp.MatchString(line):
			matches := inputBytesRegexp.FindStringSubmatch(line)
			current.InputBytes = str2float64(matches[1])
		case outputBytesRegexp.MatchString(line):
			matches := outputBytesRegexp.FindStringSubmatch(line)
			current.OutputBytes = str2float64(matches[1])
		case inputErrorsRegexp.MatchString(line):
			matches := inputErrorsRegexp.FindStringSubmatch(line)
			current.InputErrors = str2float64(matches[1])
		case outputErrorsRegexp.MatchString(line):
			matches := outputErrorsRegexp.FindStringSubmatch(line)
			current.OutputErrors = str2float64(matches[1])
		case speedRegexp.MatchString(line):
			matches := speedRegexp.FindStringSubmatch(line)
			current.SpeedString = matches[2] + " " + matches[3]
		case txNXOS.MatchString(line):
			isRx = false
		case multiBroadNXOS.MatchString(line):
			if isRx {
				matches := multiBroadNXOS.FindStringSubmatch(line)
				current.InputMulticast = str2float64(matches[1])
				current.InputBroadcast = str2float64(matches[2])
			}
		case multiBroadIOSXE.MatchString(line):
			matches := multiBroadIOSXE.FindStringSubmatch(line)
			current.InputBroadcast = str2float64(matches[1])
			current.InputMulticast = str2float64(matches[2])
		case multiBroadIOS.MatchString(line):
			matches := multiBroadIOS.FindStringSubmatch(line)
			current.InputBroadcast = str2float64(matches[1])
		}
	}

	if current != nil {
		if current.Validate() {
			interfaces = append(interfaces, current)
		} else {
			logger.Warn("Skipping invalid interface", zap.String("name", current.Name))
		}
	}

	logger.Info("Parsed interfaces", zap.Int("count", len(interfaces)))

	return interfaces
}

// parseSimpleInterfaces parses "show interface brief" output as fallback
func parseSimpleInterfaces(output string, _ *zap.Logger) []*Interface {
	interfaces := make([]*Interface, 0)

	simpleRegex := regexp.MustCompile(`^(\S+)\s+(?:\S+\s+)*(\S+)\s+(\S+)\s*$`)

	lines := strings.SplitSeq(output, "\n")

	for line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.Contains(line, "Interface") {
			continue
		}

		matches := simpleRegex.FindStringSubmatch(line)
		if len(matches) >= 4 {
			interfaceName := matches[1]
			operStatus := parseStatus(matches[3])

			iface := NewInterface(interfaceName)
			iface.OperStatus = operStatus

			interfaces = append(interfaces, iface)
		}
	}

	return interfaces
}
