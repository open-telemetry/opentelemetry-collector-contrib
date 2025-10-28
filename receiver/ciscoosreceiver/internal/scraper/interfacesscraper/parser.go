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

	AdminStatus string
	OperStatus  string

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

// Status constants for interface status
const (
	StatusUp   = "up"
	StatusDown = "down"
)

// NewInterface creates a new Interface with default values
func NewInterface(name string) *Interface {
	return &Interface{
		Name:        name,
		AdminStatus: StatusDown,
		OperStatus:  StatusDown,
	}
}

// GetOperStatusInt returns operational status as integer (1=up, 0=down)
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

// formatSpeed converts numeric speed (bps) to human-readable format with units
func formatSpeed(speedBps int64) string {
	if speedBps <= 0 {
		return ""
	}

	// Convert from bps to common network interface speeds
	switch {
	case speedBps >= 1000000000: // >= 1 Gbps
		return fmt.Sprintf("%.0f Gb/s", float64(speedBps)/1000000000)
	case speedBps >= 1000000: // >= 1 Mbps
		return fmt.Sprintf("%.0f Mb/s", float64(speedBps)/1000000)
	case speedBps >= 1000: // >= 1 Kbps
		return fmt.Sprintf("%.0f Kb/s", float64(speedBps)/1000)
	default:
		return fmt.Sprintf("%d b/s", speedBps)
	}
}

// str2float64 converts string to float64 (returns 0 for "-" or empty)
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
	// Regex patterns for parsing interface output
	txNXOS := regexp.MustCompile(`^\s+TX$`)
	// Match MAC address - handle line wrapping where (bia ...) may be on next line
	macRegexp := regexp.MustCompile(`^\s+Hardware(?: is|:) .+, address(?: is|:) ([0-9a-fA-F]{4}\.[0-9a-fA-F]{4}\.[0-9a-fA-F]{4})`)
	deviceNameRegexp := regexp.MustCompile(`^([a-zA-Z0-9/.-]+) is.*$`)
	adminStatusRegexp := regexp.MustCompile(`^.+ is (administratively)?\s*(up|down).*, line protocol is.*$`)
	adminStatusNXOSRegexp := regexp.MustCompile(`^\S+ is (up|down)(?:\s|,)?(\(Administratively down\))?.*$`)
	descRegexp := regexp.MustCompile(`^\s+Description: (.*)$`)
	dropsRegexp := regexp.MustCompile(`^\s+Input queue: \d+\/\d+\/(\d+|-)\/\d+ .+ Total output drops: (\d+|-)$`)
	// Match multicast/broadcast - handle line wrapping where "packets" may be split
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
	lines := strings.Split(output, "\n")

	logger.Debug("Processing interface parsing", zap.Int("line_count", len(lines)))

	for _, line := range lines {
		if !newIfRegexp.MatchString(line) {
			if current != nil && current.Validate() {
				interfaces = append(interfaces, current)
				logger.Debug("Added interface from parsing", zap.String("name", current.Name))
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
		if matches := adminStatusRegexp.FindStringSubmatch(line); matches != nil {
			if matches[1] == "administratively" {
				current.AdminStatus = StatusDown
			} else {
				current.AdminStatus = StatusUp
			}
			current.OperStatus = parseStatus(matches[2])
		} else if matches := adminStatusNXOSRegexp.FindStringSubmatch(line); matches != nil {
			if matches[2] == "" {
				current.AdminStatus = StatusUp
			} else {
				current.AdminStatus = StatusDown
			}
			current.OperStatus = parseStatus(matches[1])
		} else if matches := descRegexp.FindStringSubmatch(line); matches != nil {
			current.Description = matches[1]
		} else if matches := macRegexp.FindStringSubmatch(line); matches != nil {
			current.MACAddress = matches[1]
			logger.Debug("Parsed MAC address for interface",
				zap.String("interface", current.Name),
				zap.String("mac", current.MACAddress))
		} else if matches := dropsRegexp.FindStringSubmatch(line); matches != nil {
			current.InputDrops = str2float64(matches[1])
			current.OutputDrops = str2float64(matches[2])
		} else if matches := inputBytesRegexp.FindStringSubmatch(line); matches != nil {
			current.InputBytes = str2float64(matches[1])
		} else if matches := outputBytesRegexp.FindStringSubmatch(line); matches != nil {
			current.OutputBytes = str2float64(matches[1])
		} else if matches := inputErrorsRegexp.FindStringSubmatch(line); matches != nil {
			current.InputErrors = str2float64(matches[1])
		} else if matches := outputErrorsRegexp.FindStringSubmatch(line); matches != nil {
			current.OutputErrors = str2float64(matches[1])
		} else if matches := speedRegexp.FindStringSubmatch(line); matches != nil {
			current.SpeedString = matches[2] + " " + matches[3]
		} else if txNXOS.MatchString(line) {
			isRx = false
		} else if matches := multiBroadNXOS.FindStringSubmatch(line); matches != nil {
			if isRx {
				current.InputMulticast = str2float64(matches[1])
				current.InputBroadcast = str2float64(matches[2])
				logger.Debug("Parsed NX-OS multicast/broadcast",
					zap.String("interface", current.Name),
					zap.Float64("multicast", current.InputMulticast),
					zap.Float64("broadcast", current.InputBroadcast))
			}
		} else if matches := multiBroadIOSXE.FindStringSubmatch(line); matches != nil {
			current.InputBroadcast = str2float64(matches[1])
			current.InputMulticast = str2float64(matches[2])
			logger.Debug("Parsed IOS XE multicast/broadcast",
				zap.String("interface", current.Name),
				zap.Float64("multicast", current.InputMulticast),
				zap.Float64("broadcast", current.InputBroadcast))
		} else if matches := multiBroadIOS.FindStringSubmatch(line); matches != nil {
			current.InputBroadcast = str2float64(matches[1])
			logger.Debug("Parsed IOS broadcast",
				zap.String("interface", current.Name),
				zap.Float64("broadcast", current.InputBroadcast))
		}
	}

	if current != nil {
		if current.Validate() {
			interfaces = append(interfaces, current)
			logger.Debug("Added final interface from parsing",
				zap.String("name", current.Name),
				zap.String("mac", current.MACAddress),
				zap.String("oper_status", current.OperStatus),
				zap.String("admin_status", current.AdminStatus),
				zap.Bool("has_mac", current.MACAddress != ""))
		} else {
			logger.Warn("Skipping invalid interface",
				zap.String("name", current.Name))
		}
	}

	withMAC := 0
	withoutMAC := 0
	withMulticast := 0
	withBroadcast := 0
	for _, intf := range interfaces {
		if intf.MACAddress != "" {
			withMAC++
		} else {
			withoutMAC++
			logger.Debug("Interface without MAC address",
				zap.String("interface", intf.Name),
				zap.String("type", "virtual or no hardware"))
		}
		if intf.InputMulticast > 0 {
			withMulticast++
		}
		if intf.InputBroadcast > 0 {
			withBroadcast++
		}
		if intf.InputMulticast == 0 && intf.InputBroadcast == 0 {
			logger.Debug("Interface with zero multicast/broadcast counters",
				zap.String("interface", intf.Name))
		}
	}
	logger.Info("Interface parsing complete",
		zap.Int("total_interfaces", len(interfaces)),
		zap.Int("with_mac", withMAC),
		zap.Int("without_mac", withoutMAC),
		zap.Int("with_multicast", withMulticast),
		zap.Int("with_broadcast", withBroadcast))

	return interfaces
}

// parseSimpleInterfaces implements simple interface parsing fallback (for "show interface brief" output)
func parseSimpleInterfaces(output string, logger *zap.Logger) []*Interface {
	interfaces := make([]*Interface, 0)

	// Simple regex for "show interface brief" output format
	// Example: Ethernet1/1   10.1.1.1   YES   NVRAM  up     up
	simpleRegex := regexp.MustCompile(`^(\S+)\s+(?:\S+\s+)*(\S+)\s+(\S+)\s*$`)

	lines := strings.Split(output, "\n")
	logger.Debug("Parsing simple interface format", zap.Int("lines", len(lines)))

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.Contains(line, "Interface") {
			continue
		}

		matches := simpleRegex.FindStringSubmatch(line)
		if len(matches) >= 4 {
			interfaceName := matches[1]
			adminStatus := parseStatus(matches[2])
			operStatus := parseStatus(matches[3])

			iface := NewInterface(interfaceName)
			iface.AdminStatus = adminStatus
			iface.OperStatus = operStatus

			logger.Debug("Parsed simple interface",
				zap.String("name", interfaceName),
				zap.String("admin", adminStatus),
				zap.String("oper", operStatus))

			interfaces = append(interfaces, iface)
		}
	}

	logger.Info("Simple interface parsing completed",
		zap.Int("interfaces_found", len(interfaces)))

	return interfaces
}
