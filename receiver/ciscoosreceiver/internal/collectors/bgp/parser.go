// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package bgp // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ciscoosreceiver/internal/collectors/bgp"

import (
	"regexp"
	"strconv"
	"strings"
)

// Parser handles parsing of BGP command output
type Parser struct {
	neighborPattern *regexp.Regexp
}

// NewParser creates a new BGP parser
func NewParser() *Parser {
	pattern := regexp.MustCompile(`(\S+)\s+\d+\s+(\d+)\s+(\d+)\s+(\d+)\s+\d+\s+\d+\s+\d+\s+\S+\s+(\S+)`)
	return &Parser{
		neighborPattern: pattern,
	}
}

// ParseBGPSummary parses the output of "show bgp all summary" command
func (p *Parser) ParseBGPSummary(output string) ([]*Session, error) {
	var sessions []*Session

	lines := strings.Split(output, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		matches := p.neighborPattern.FindStringSubmatch(line)
		if len(matches) != 6 {
			continue
		}

		neighborIP := matches[1]
		asn := matches[2]
		inputMsgsStr := matches[3]
		outputMsgsStr := matches[4]
		prefixesOrStateStr := matches[5]

		inputMsgs, err := strconv.ParseInt(inputMsgsStr, 10, 64)
		if err != nil {
			continue
		}

		outputMsgs, err := strconv.ParseInt(outputMsgsStr, 10, 64)
		if err != nil {
			continue
		}

		var prefixes int64 = 0
		var sessionUp bool = false

		if prefixesNum, err := strconv.ParseInt(prefixesOrStateStr, 10, 64); err == nil {
			prefixes = prefixesNum
			sessionUp = true
		} else {
			prefixes = 0
			sessionUp = false
		}

		session := NewSession(neighborIP, asn)
		session.SetMessageCounts(inputMsgs, outputMsgs)

		if sessionUp {
			session.SetPrefixesReceived(prefixes)
		} else {
			session.SetPrefixesReceived(-1)
		}

		if session.Validate() {
			sessions = append(sessions, session)
		}
	}

	return sessions, nil
}

// ParseBGPNeighborDetail parses detailed BGP neighbor information
func (p *Parser) ParseBGPNeighborDetail(output string) (*Session, error) {
	return nil, nil
}

// GetSupportedCommands returns the commands this parser can handle
func (p *Parser) GetSupportedCommands() []string {
	return []string{
		"show bgp all summary",
		"show bgp ipv4 unicast summary",
		"show bgp ipv6 unicast summary",
	}
}

// ValidateOutput checks if the output looks like valid BGP summary output
func (p *Parser) ValidateOutput(output string) bool {
	// Check for common BGP summary indicators
	indicators := []string{
		"BGP router identifier",
		"BGP table version",
		"Neighbor",
		"AS MsgRcvd MsgSent",
	}

	for _, indicator := range indicators {
		if strings.Contains(output, indicator) {
			return true
		}
	}

	return false
}
