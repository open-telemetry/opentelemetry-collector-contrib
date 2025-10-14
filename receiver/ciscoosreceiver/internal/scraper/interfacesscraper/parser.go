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

	InputPkts  float64
	OutputPkts float64

	InputBroadcast float64
	InputMulticast float64

	Speed       int64
	SpeedString string
	MTU         int64
	VLANs       []string
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
		VLANs:       make([]string, 0),
	}
}

// GetAdminStatusInt returns admin status as integer (1=up, 0=down)
func (i *Interface) GetAdminStatusInt() int64 {
	if i.AdminStatus == StatusUp {
		return 1
	}
	return 0
}

// GetOperStatusInt returns operational status as integer (1=up, 0=down)
func (i *Interface) GetOperStatusInt() int64 {
	if i.OperStatus == StatusUp {
		return 1
	}
	return 0
}

// GetErrorStatusInt returns error status as integer (1=error, 0=no error)
func (i *Interface) GetErrorStatusInt() int64 {
	if i.AdminStatus != i.OperStatus {
		return 1
	}
	return 0
}

// IsValid returns true if interface has minimum required data
func (i *Interface) IsValid() bool {
	return i.Name != ""
}

// Validate checks if the interface has valid data
func (i *Interface) Validate() bool {
	return i.Name != ""
}

// parseStatus converts string status to status string
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

// str2float64 converts string to float64, handling "-" as 0
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
	macRegexp := regexp.MustCompile(`^\s+Hardware(?: is|:) .+, address(?: is|:) (.*) \(.*\)$`)
	deviceNameRegexp := regexp.MustCompile(`^([a-zA-Z0-9/.-]+) is.*$`)
	adminStatusRegexp := regexp.MustCompile(`^.+ is (administratively)?\s*(up|down).*, line protocol is.*$`)
	adminStatusNXOSRegexp := regexp.MustCompile(`^\S+ is (up|down)(?:\s|,)?(\(Administratively down\))?.*$`)
	descRegexp := regexp.MustCompile(`^\s+Description: (.*)$`)
	dropsRegexp := regexp.MustCompile(`^\s+Input queue: \d+\/\d+\/(\d+|-)\/\d+ .+ Total output drops: (\d+|-)$`)
	multiBroadNXOS := regexp.MustCompile(`^.* (\d+|-) multicast packets\s+(\d+|-) broadcast packets$`)
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
			// Combine speed number + units
			current.SpeedString = matches[2] + " " + matches[3]
		} else if txNXOS.MatchString(line) {
			isRx = false
		} else if matches := multiBroadNXOS.FindStringSubmatch(line); matches != nil {
			if isRx {
				current.InputMulticast = str2float64(matches[1])
				current.InputBroadcast = str2float64(matches[2])
			}
		} else if matches := multiBroadIOSXE.FindStringSubmatch(line); matches != nil {
			current.InputBroadcast = str2float64(matches[1])
			current.InputMulticast = str2float64(matches[2])
		} else if matches := multiBroadIOS.FindStringSubmatch(line); matches != nil {
			current.InputBroadcast = str2float64(matches[1])
		}
	}

	// Add the last interface if it exists and is valid
	if current != nil && current.Validate() {
		interfaces = append(interfaces, current)
		logger.Debug("Added final interface from parsing", zap.String("name", current.Name))
	}

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
