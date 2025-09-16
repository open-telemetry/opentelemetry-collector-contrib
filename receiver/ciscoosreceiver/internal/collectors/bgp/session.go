// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package bgp // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ciscoosreceiver/internal/collectors/bgp"

import "fmt"

// Session represents a BGP session with a neighbor
type Session struct {
	NeighborIP       string
	ASN              string
	State            string
	PrefixesReceived int64
	MessagesInput    int64
	MessagesOutput   int64
	IsUp             bool
}

// NewSession creates a new BGP session
func NewSession(neighborIP, asn string) *Session {
	return &Session{
		NeighborIP: neighborIP,
		ASN:        asn,
		State:      "Unknown",
		IsUp:       false,
	}
}

// SetPrefixesReceived sets the number of prefixes received and determines session state
func (s *Session) SetPrefixesReceived(prefixes int64) {
	s.PrefixesReceived = prefixes

	// Session is considered up if prefixes >= 0, down if negative
	if prefixes >= 0 {
		s.IsUp = true
		s.State = "Established"
	} else {
		s.IsUp = false
		s.State = "Idle"
		s.PrefixesReceived = 0 // Don't report negative values
	}
}

// SetMessageCounts sets the input and output message counts
func (s *Session) SetMessageCounts(input, output int64) {
	s.MessagesInput = input
	s.MessagesOutput = output
}

// GetUpStatus returns 1 if session is up, 0 if down
func (s *Session) GetUpStatus() int64 {
	if s.IsUp {
		return 1
	}
	return 0
}

// String returns a string representation of the session
func (s *Session) String() string {
	return fmt.Sprintf("BGP Session %s (AS%s): %s, Prefixes: %d, In: %d, Out: %d",
		s.NeighborIP, s.ASN, s.State, s.PrefixesReceived, s.MessagesInput, s.MessagesOutput)
}

// Validate checks if the session has valid data
func (s *Session) Validate() bool {
	return s.NeighborIP != "" && s.ASN != ""
}
