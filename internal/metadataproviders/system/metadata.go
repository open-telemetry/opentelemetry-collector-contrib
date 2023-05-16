// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package system // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/metadataproviders/system"

import (
	"context"
	"fmt"
	"net"
	"os"
	"runtime"
	"strings"

	"github.com/Showmax/go-fqdn"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
	"go.opentelemetry.io/otel/sdk/resource"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/metadataproviders/internal"
)

// nameInfoProvider abstracts domain name resolution so it can be swapped for
// testing
type nameInfoProvider struct {
	osHostname  func() (string, error)
	lookupCNAME func(string) (string, error)
	lookupHost  func(string) ([]string, error)
	lookupAddr  func(string) ([]string, error)
}

// newNameInfoProvider creates a name info provider for production use, using
// DNS to resolve domain names
func newNameInfoProvider() nameInfoProvider {
	return nameInfoProvider{
		osHostname:  os.Hostname,
		lookupCNAME: net.LookupCNAME,
		lookupHost:  net.LookupHost,
		lookupAddr:  net.LookupAddr,
	}
}

type Provider interface {
	// Hostname returns the OS hostname
	Hostname() (string, error)

	// FQDN returns the fully qualified domain name
	FQDN() (string, error)

	// OSType returns the host operating system
	OSType() (string, error)

	// LookupCNAME returns the canonical name for the current host
	LookupCNAME() (string, error)

	// ReverseLookupHost does a reverse DNS query on the current host's IP address
	ReverseLookupHost() (string, error)

	// HostID returns Host Unique Identifier
	HostID(ctx context.Context) (string, error)
}

type systemMetadataProvider struct {
	nameInfoProvider
}

func NewProvider() Provider {
	return systemMetadataProvider{nameInfoProvider: newNameInfoProvider()}
}

func (systemMetadataProvider) OSType() (string, error) {
	return internal.GOOSToOSType(runtime.GOOS), nil
}

func (systemMetadataProvider) FQDN() (string, error) {
	return fqdn.FqdnHostname()
}

func (p systemMetadataProvider) Hostname() (string, error) {
	return p.nameInfoProvider.osHostname()
}

func (p systemMetadataProvider) LookupCNAME() (string, error) {
	hostname, err := p.Hostname()
	if err != nil {
		return "", fmt.Errorf("LookupCNAME failed to get hostname: %w", err)
	}
	cname, err := p.nameInfoProvider.lookupCNAME(hostname)
	if err != nil {
		return "", fmt.Errorf("LookupCNAME failed to get CNAME: %w", err)
	}
	return strings.TrimRight(cname, "."), nil
}

func (p systemMetadataProvider) ReverseLookupHost() (string, error) {
	hostname, err := p.Hostname()
	if err != nil {
		return "", fmt.Errorf("ReverseLookupHost failed to get hostname: %w", err)
	}
	return p.hostnameToDomainName(hostname)
}

func (p systemMetadataProvider) hostnameToDomainName(hostname string) (string, error) {
	ipAddresses, err := p.nameInfoProvider.lookupHost(hostname)
	if err != nil {
		return "", fmt.Errorf("hostnameToDomainName failed to convert hostname to IP addresses: %w", err)
	}
	return p.reverseLookup(ipAddresses)
}

func (p systemMetadataProvider) reverseLookup(ipAddresses []string) (string, error) {
	var err error
	for _, ip := range ipAddresses {
		var names []string
		names, err = p.nameInfoProvider.lookupAddr(ip)
		if err != nil {
			continue
		}
		return strings.TrimRight(names[0], "."), nil
	}
	return "", fmt.Errorf("reverseLookup failed to convert IP addresses to name: %w", err)
}

func (p systemMetadataProvider) HostID(ctx context.Context) (string, error) {
	res, err := resource.New(ctx,
		resource.WithHostID(),
	)

	if err != nil {
		return "", fmt.Errorf("failed to obtain host id: %w", err)
	}

	iter := res.Iter()

	for iter.Next() {
		if iter.Attribute().Key == conventions.AttributeHostID {
			return iter.Attribute().Value.Emit(), nil
		}
	}

	return "", fmt.Errorf("failed to obtain host id")
}
