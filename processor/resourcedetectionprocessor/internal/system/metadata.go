// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package system // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/system"

import (
	"fmt"
	"net"
	"os"
	"runtime"

	"github.com/Showmax/go-fqdn"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
)

type systemMetadata interface {
	// Hostname returns the OS hostname
	Hostname() (string, error)

	// FQDN returns the fully qualified domain name
	FQDN() (string, error)

	// OSType returns the host operating system
	OSType() (string, error)

	LookupCNAME() (string, error)

	ReverseLookupHost() (string, error)
}

type systemMetadataImpl struct{}

func (*systemMetadataImpl) OSType() (string, error) {
	return internal.GOOSToOSType(runtime.GOOS), nil
}

func (*systemMetadataImpl) FQDN() (string, error) {
	return fqdn.FqdnHostname()
}

func (*systemMetadataImpl) Hostname() (string, error) {
	return os.Hostname()
}

func (m *systemMetadataImpl) LookupCNAME() (string, error) {
	hostname, err := m.Hostname()
	if err != nil {
		return "", fmt.Errorf("LookupCNAME failed to get hostname: %w", err)
	}
	cname, err := net.LookupCNAME(hostname)
	if err != nil {
		return "", fmt.Errorf("LookupCNAME failed to get CNAME: %w", err)
	}
	return stripTrailingDot(cname), nil
}

func (m *systemMetadataImpl) ReverseLookupHost() (string, error) {
	hostname, err := m.Hostname()
	if err != nil {
		return "", fmt.Errorf("ReverseLookupHost failed to get hostname: %w", err)
	}
	return hostnameToDomainName(hostname)
}

func hostnameToDomainName(hostname string) (string, error) {
	ipAddresses, err := net.LookupHost(hostname)
	if err != nil {
		return "", fmt.Errorf("hostnameToDomainName failed to convert hostname to IP addresses: %w", err)
	}
	return reverseLookup(ipAddresses)
}

func reverseLookup(ipAddresses []string) (string, error) {
	var err error
	for _, ip := range ipAddresses {
		var names []string
		names, err = net.LookupAddr(ip)
		if err != nil {
			continue
		}
		return stripTrailingDot(names[0]), nil
	}
	return "", fmt.Errorf("reverseLookup failed to convert IP address to name: %w", err)
}

func stripTrailingDot(name string) string {
	nameLen := len(name)
	lastIdx := nameLen - 1
	if nameLen > 0 && name[lastIdx] == '.' {
		name = name[:lastIdx]
	}
	return name
}
