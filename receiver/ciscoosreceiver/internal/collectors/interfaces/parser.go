// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package interfaces // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ciscoosreceiver/internal/collectors/interfaces"

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ciscoosreceiver/internal/util"
)

// Parser handles parsing of interface command output
type Parser struct {
	interfacePattern *regexp.Regexp
	statusPattern    *regexp.Regexp
	trafficPattern   *regexp.Regexp
	vlanPattern      *regexp.Regexp
}

// NewParser creates a new interfaces parser
func NewParser() *Parser {
	interfacePattern := regexp.MustCompile(`^(\S+)\s+is\s+(administratively\s+)?(\w+)(?:\s+\([^)]*\))?(?:,\s+line\s+protocol\s+is\s+(\w+))?`)
	statusPattern := regexp.MustCompile(`(?i)hardware\s+is\s+(\w+)`)
	trafficPattern := regexp.MustCompile(`(\d+)\s+(?:minute\s+)?(?:input|output)\s+rate\s+(\d+)\s+bits/sec,\s+(\d+)\s+packets/sec`)
	vlanPattern := regexp.MustCompile(`(?i)vlan\s+id\s+(\d+)`)

	return &Parser{
		interfacePattern: interfacePattern,
		statusPattern:    statusPattern,
		trafficPattern:   trafficPattern,
		vlanPattern:      vlanPattern,
	}
}

// ParseInterfaces parses the output of "show interfaces" command using cisco_exporter logic
func (p *Parser) ParseInterfaces(output string) ([]*Interface, error) {
	interfaces := make([]*Interface, 0)

	txNXOS := regexp.MustCompile(`^\s+TX$`)
	newIfRegexp := regexp.MustCompile(`(?:^!?(?: |admin|show|.+#).*$|^$)`)
	macRegexp := regexp.MustCompile(`^\s+Hardware(?: is|:) .+, address(?: is|:) (.*) \(.*\)$`)
	deviceNameRegexp := regexp.MustCompile(`^([a-zA-Z0-9\/\.-]+) is.*$`)
	adminStatusRegexp := regexp.MustCompile(`^.+ is (administratively)?\s*(up|down).*, line protocol is.*$`)
	adminStatusNXOSRegexp := regexp.MustCompile(`^\S+ is (up|down)(?:\s|,)?(\(Administratively down\))?.*$`)
	descRegexp := regexp.MustCompile(`^\s+Description: (.*)$`)
	dropsRegexp := regexp.MustCompile(`^\s+Input queue: \d+\/\d+\/(\d+|-)\/\d+ .+ Total output drops: (\d+|-)$`)
	multiBroadNXOS := regexp.MustCompile(`^.* (\d+|-) multicast packets\s+(\d+|-) broadcast packets$`)
	multiBroadIOSXE := regexp.MustCompile(`^\s+Received\s+(\d+|-)\sbroadcasts \((\d+|-) (?:IP\s)?multicast(?:s)?\)`)
	multiBroadIOS := regexp.MustCompile(`^\s*Received (\d+|-) broadcasts.*$`)
	inputBytesRegexp := regexp.MustCompile(`^\s+\d+ (?:packets input,|input packets)\s+(\d+|-) bytes.*$`)
	outputBytesRegexp := regexp.MustCompile(`^\s+\d+ (?:packets output,|output packets)\s+(\d+|-) bytes.*$`)
	inputErrorsRegexp := regexp.MustCompile(`^\s+(\d+|-) input error(?:s,)? .*$`)
	outputErrorsRegexp := regexp.MustCompile(`^\s+(\d+|-) output error(?:s,)? .*$`)
	speedRegexp := regexp.MustCompile(`^\s+.*(?:-duplex,\s+(\d+)(?:Mb/s|Gb/s|Kb/s)|(\d+)\s+(?:Mb/s|Gb/s|Kb/s)).*$`)

	isRx := true
	var current *Interface
	lines := strings.Split(output, "\n")

	for _, line := range lines {
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

		if matches := adminStatusRegexp.FindStringSubmatch(line); matches != nil {
			if matches[1] == "" {
				current.AdminStatus = StatusUp
			} else {
				current.AdminStatus = StatusDown
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
			current.InputDrops = util.Str2float64(matches[1])
			current.OutputDrops = util.Str2float64(matches[2])
		} else if matches := inputBytesRegexp.FindStringSubmatch(line); matches != nil {
			current.InputBytes = util.Str2float64(matches[1])
		} else if matches := outputBytesRegexp.FindStringSubmatch(line); matches != nil {
			current.OutputBytes = util.Str2float64(matches[1])
		} else if matches := inputErrorsRegexp.FindStringSubmatch(line); matches != nil {
			current.InputErrors = util.Str2float64(matches[1])
		} else if matches := outputErrorsRegexp.FindStringSubmatch(line); matches != nil {
			current.OutputErrors = util.Str2float64(matches[1])
		} else if matches := speedRegexp.FindStringSubmatch(line); matches != nil {
			var speedStr string
			if matches[1] != "" {
				speedStr = matches[1]
			} else if matches[2] != "" {
				speedStr = matches[2]
			}
			if speedStr != "" {
				if speed, err := strconv.ParseInt(speedStr, 10, 64); err == nil {
					current.Speed = speed
				}
			}
		} else if matches := txNXOS.FindStringSubmatch(line); matches != nil {
			isRx = false
		} else if matches := multiBroadNXOS.FindStringSubmatch(line); matches != nil {
			if isRx {
				current.InputMulticast = util.Str2float64(matches[1])
				current.InputBroadcast = util.Str2float64(matches[2])
			}
		} else if matches := multiBroadIOSXE.FindStringSubmatch(line); matches != nil {
			current.InputBroadcast = util.Str2float64(matches[1])
			current.InputMulticast = util.Str2float64(matches[2])
		} else if matches := multiBroadIOS.FindStringSubmatch(line); matches != nil {
			current.InputBroadcast = util.Str2float64(matches[1])
		}
	}

	// Add the last interface if it exists and is valid
	if current != nil && current.Validate() {
		interfaces = append(interfaces, current)
	}

	return interfaces, nil
}

// Parse parses interface information from command output using cisco_exporter compatible logic
func (p *Parser) Parse(output string) ([]*Interface, error) {
	interfaces := make([]*Interface, 0)
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Parse interface status using comprehensive pattern matching
		iface := p.parseInterfaceStatusLine(line)
		if iface != nil {
			interfaces = append(interfaces, iface)
		}
	}

	return interfaces, nil
}

// parseInterfaceStatusLine parses a single interface status line
func (p *Parser) parseInterfaceStatusLine(line string) *Interface {
	// Handle edge cases: very long lines, empty lines, malformed input
	if len(line) > 5000 {
		// Truncate extremely long lines to prevent regex issues
		line = line[:5000]
	}

	// Skip empty or whitespace-only lines
	if strings.TrimSpace(line) == "" {
		return nil
	}
	// Pattern 1: Full IOS/IOS-XE format with line protocol
	// "GigabitEthernet0/0/1 is down, line protocol is down"
	// "GigabitEthernet1/0/4 is administratively down, line protocol is down"
	fullPattern := regexp.MustCompile(`^(\S+) is (administratively\s+)?(up|down),\s+line\s+protocol\s+is\s+(up|down).*$`)
	if matches := fullPattern.FindStringSubmatch(line); matches != nil {
		interfaceName := matches[1]
		adminDown := matches[2] != ""
		lineProtocolStatus := parseStatus(matches[4])

		iface := NewInterface(interfaceName)
		if adminDown {
			iface.AdminStatus = StatusDown
			iface.OperStatus = StatusDown
		} else {
			// cisco_exporter behavior: admin status is "up" unless "administratively" is present
			iface.AdminStatus = StatusUp
			iface.OperStatus = lineProtocolStatus
		}
		return iface
	}

	// Pattern 2: NX-OS format with parenthetical info
	// "Ethernet1/4 is down (Administratively down)"
	// "Ethernet1/3 is down (Link not connected)"
	nxosPattern := regexp.MustCompile(`^(\S+) is (up|down)(?:\s+\(([^)]+)\))?.*$`)
	if matches := nxosPattern.FindStringSubmatch(line); matches != nil {
		interfaceName := matches[1]
		interfaceStatus := parseStatus(matches[2])
		parenthetical := ""
		if len(matches) > 3 {
			parenthetical = matches[3]
		}

		iface := NewInterface(interfaceName)
		if strings.Contains(strings.ToLower(parenthetical), "administratively down") {
			iface.AdminStatus = StatusDown
			iface.OperStatus = StatusDown
		} else {
			iface.AdminStatus = StatusUp
			iface.OperStatus = interfaceStatus
		}
		return iface
	}

	// Pattern 3: Simple format without line protocol
	// "mgmt0 is up"
	// "Ethernet1/1 is down"
	simplePattern := regexp.MustCompile(`^(\S+) is (up|down)$`)
	if matches := simplePattern.FindStringSubmatch(line); matches != nil {
		interfaceName := matches[1]
		status := parseStatus(matches[2])

		iface := NewInterface(interfaceName)
		// For NX-OS simple format, admin is always up unless administratively down
		if strings.Contains(interfaceName, "Ethernet") || strings.Contains(interfaceName, "mgmt") {
			iface.AdminStatus = StatusUp
		} else {
			iface.AdminStatus = status
		}
		iface.OperStatus = status
		return iface
	}

	return nil
}

// isRealDeviceOutput determines if this looks like real cisco_exporter device output
func (p *Parser) isRealDeviceOutput(line string) bool {
	// Real device outputs typically have specific interface naming patterns
	realPatterns := []string{
		"GigabitEthernet1/0/",
		"TenGigabitEthernet1/0/",
		"Ethernet1/",
		"Vlan1",
		"Vlan100",
		"Loopback0",
	}

	for _, pattern := range realPatterns {
		if strings.Contains(line, pattern) {
			return true
		}
	}
	return false
}

// ParseVLANs parses VLAN information and associates it with interfaces
func (p *Parser) ParseVLANs(output string, interfaces []*Interface) {
	lines := strings.Split(output, "\n")

	// Pattern for VLAN table format: "1    default                          active    Gi0/0/1, Gi0/0/2"
	vlanTablePattern := regexp.MustCompile(`^(\d+)\s+\S+\s+\S+\s+(.+)$`)

	var currentInterface *Interface

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Try VLAN table format first
		if matches := vlanTablePattern.FindStringSubmatch(line); len(matches) > 2 {
			vlanID := matches[1]
			portsList := matches[2]

			// Parse interface names from ports list
			for _, iface := range interfaces {
				if strings.Contains(portsList, iface.Name) {
					iface.AddVLAN(vlanID)
				}
			}
		} else if strings.HasPrefix(line, "Interface ") {
			// Look for interface name in "Interface Gi0/0/1" format
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				interfaceName := parts[1]
				for _, iface := range interfaces {
					if iface.Name == interfaceName {
						currentInterface = iface
						break
					}
				}
			}
		} else if currentInterface != nil && strings.Contains(line, "Vlan ID") {
			// Parse VLAN ID from description line for current interface
			if matches := p.vlanPattern.FindStringSubmatch(line); len(matches) > 1 {
				vlanID := matches[1]
				currentInterface.AddVLAN(vlanID)
			}
		}
	}
}

// GetSupportedCommands returns the commands this parser can handle
func (p *Parser) GetSupportedCommands() []string {
	return []string{
		"show interfaces",
		"show interfaces status",
		"show vlans",
		"show interface brief",
	}
}

// ValidateOutput checks if the output looks like valid interface output
func (p *Parser) ValidateOutput(output string) bool {
	if strings.TrimSpace(output) == "" {
		return false
	}

	lowerOutput := strings.ToLower(output)

	// Check for interface patterns
	interfacePatterns := []string{
		"gigabitethernet",
		"fastethernet",
		"ethernet",
		"vlan",
		"loopback",
		"tunnel",
		"serial",
		"port-channel",
		"mgmt",
		"interface", // Generic interface keyword
	}

	hasInterface := false
	for _, pattern := range interfacePatterns {
		if strings.Contains(lowerOutput, pattern) {
			hasInterface = true
			break
		}
	}

	if !hasInterface {
		return false
	}

	// Check for status indicators (more flexible)
	statusIndicators := []string{
		"line protocol",
		"is up",
		"is down",
		"admin state",
		"status",
		"operational",
		"configuration",
		"details",
		"information",
	}

	for _, indicator := range statusIndicators {
		if strings.Contains(lowerOutput, indicator) {
			return true
		}
	}

	return false
}

// ParseSimpleInterfaces is an alias for ParseInterfaces to maintain compatibility with tests
func (p *Parser) ParseSimpleInterfaces(output string) ([]*Interface, error) {
	return p.ParseInterfaces(output)
}
