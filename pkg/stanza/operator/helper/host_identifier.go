// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package helper

import (
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/open-telemetry/opentelemetry-log-collection/entry"
	"github.com/open-telemetry/opentelemetry-log-collection/errors"
)

// NewHostIdentifierConfig returns a HostIdentifierConfig with default values
func NewHostIdentifierConfig() HostIdentifierConfig {
	return HostIdentifierConfig{
		IncludeHostname: true,
		IncludeIP:       true,
		getHostname:     getHostname,
		getIP:           getIP,
	}
}

// HostIdentifierConfig is the configuration of a host identifier
type HostIdentifierConfig struct {
	IncludeHostname bool `json:"include_hostname,omitempty"     yaml:"include_hostname,omitempty"`
	IncludeIP       bool `json:"include_ip,omitempty"     yaml:"include_ip,omitempty"`
	getHostname     func() (string, error)
	getIP           func() (string, error)
}

// Build will build a host attributer from the supplied configuration
func (c HostIdentifierConfig) Build() (HostIdentifier, error) {
	identifier := HostIdentifier{
		includeHostname: c.IncludeHostname,
		includeIP:       c.IncludeIP,
	}

	if c.getHostname == nil {
		return identifier, fmt.Errorf("getHostname func is not set")
	}

	if c.getIP == nil {
		return identifier, fmt.Errorf("getIP func is not set")
	}

	if c.IncludeHostname {
		hostname, err := c.getHostname()
		if err != nil {
			return identifier, errors.Wrap(err, "get hostname")
		}
		identifier.hostname = hostname
	}

	if c.IncludeIP {
		ip, err := c.getIP()
		if err != nil {
			return identifier, errors.Wrap(err, "get ip address")
		}
		identifier.ip = ip
	}

	return identifier, nil
}

// getHostname will return the hostname of the current host
func getHostname() (string, error) {
	return os.Hostname()
}

// getIP will return the IP address of the current host
func getIP() (string, error) {
	var ip string

	interfaces, err := net.Interfaces()
	if err != nil {
		return "", errors.Wrap(err, "list interfaces")
	}

	for _, i := range interfaces {
		// Skip loopback interfaces
		if i.Flags&net.FlagLoopback != 0 {
			continue
		}

		// Skip down interfaces
		if i.Flags&net.FlagUp == 0 {
			continue
		}

		addrs, err := i.Addrs()
		if err != nil {
			continue
		}
		if len(addrs) > 0 {
			ip = strings.Split(addrs[0].String(), "/")[0]
		}
	}

	if len(ip) == 0 {
		return "", errors.NewError(
			"failed to find ip address",
			"check that a non-loopback interface with an assigned IP address exists and is running",
		)
	}

	return ip, nil
}

// HostIdentifier is a helper that adds host related metadata to an entry's resource
type HostIdentifier struct {
	hostname        string
	ip              string
	includeHostname bool
	includeIP       bool
}

// Identify will add host related metadata to an entry's resource
func (h *HostIdentifier) Identify(entry *entry.Entry) {
	if h.includeHostname {
		entry.AddResourceKey("host.name", h.hostname)
	}

	if h.includeIP {
		entry.AddResourceKey("host.ip", h.ip)
	}
}
