// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package system // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/metadataproviders/system"

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"net"
	"os"
	"runtime"
	"strings"

	"github.com/Showmax/go-fqdn"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/host"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/resource"
	conventions "go.opentelemetry.io/otel/semconv/v1.38.0"

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

	// OSDescription returns a human readable description of the OS.
	OSDescription(ctx context.Context) (string, error)

	// OSType returns the host operating system
	OSType() (string, error)

	// OSVersion returns the version of the operating system
	OSVersion() (string, error)

	// LookupCNAME returns the canonical name for the current host
	LookupCNAME() (string, error)

	// ReverseLookupHost does a reverse DNS query on the current host's IP address
	ReverseLookupHost() (string, error)

	// HostID returns Host Unique Identifier
	HostID(ctx context.Context) (string, error)

	// HostArch returns the host architecture
	HostArch() (string, error)

	// HostIPs returns the host's IP interfaces
	HostIPs() ([]net.IP, error)

	// HostMACs returns the host's MAC addresses
	HostMACs() ([]net.HardwareAddr, error)

	// CPUInfo returns the host's CPU info
	CPUInfo(ctx context.Context) ([]cpu.InfoStat, error)
	// OSName returns the OS name according to semantic conventions.
	OSName(ctx context.Context) (string, error)

	// OSBuildID returns the OS build ID according to semantic conventions.
	OSBuildID(ctx context.Context) (string, error)

	// HostInterfaces returns the host's Interfaces info
	HostInterfaces() ([]net.Interface, error)
}

type systemMetadataProvider struct {
	nameInfoProvider
	newResource func(context.Context, ...resource.Option) (*resource.Resource, error)
}

func NewProvider() Provider {
	return systemMetadataProvider{nameInfoProvider: newNameInfoProvider(), newResource: resource.New}
}

func (systemMetadataProvider) OSType() (string, error) {
	return internal.GOOSToOSType(runtime.GOOS), nil
}

func (systemMetadataProvider) OSVersion() (string, error) {
	info, err := host.Info()
	if err != nil {
		return "", fmt.Errorf("OSVersion failed to get os version: %w", err)
	}
	return info.PlatformVersion, nil
}

func (systemMetadataProvider) FQDN() (string, error) {
	return fqdn.FqdnHostname()
}

func (p systemMetadataProvider) Hostname() (string, error) {
	return p.osHostname()
}

func (p systemMetadataProvider) LookupCNAME() (string, error) {
	hostname, err := p.Hostname()
	if err != nil {
		return "", fmt.Errorf("LookupCNAME failed to get hostname: %w", err)
	}
	cname, err := p.lookupCNAME(hostname)
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
	ipAddresses, err := p.lookupHost(hostname)
	if err != nil {
		return "", fmt.Errorf("hostnameToDomainName failed to convert hostname to IP addresses: %w", err)
	}
	return p.reverseLookup(ipAddresses)
}

func (p systemMetadataProvider) reverseLookup(ipAddresses []string) (string, error) {
	var err error
	for _, ip := range ipAddresses {
		var names []string
		names, err = p.lookupAddr(ip)
		if err != nil {
			continue
		}
		return strings.TrimRight(names[0], "."), nil
	}
	return "", fmt.Errorf("reverseLookup failed to convert IP addresses to name: %w", err)
}

func (p systemMetadataProvider) fromOption(ctx context.Context, opt resource.Option, semconv string) (string, error) {
	res, err := p.newResource(ctx, opt)
	if err != nil {
		return "", fmt.Errorf("failed to obtain %q: %w", semconv, err)
	}

	iter := res.Iter()
	for iter.Next() {
		if iter.Attribute().Key == attribute.Key(semconv) {
			v := iter.Attribute().Value.Emit()

			if v == "" {
				return "", fmt.Errorf("empty %q", semconv)
			}
			return v, nil
		}
	}

	return "", fmt.Errorf("failed to obtain %q", semconv)
}

func (p systemMetadataProvider) HostID(ctx context.Context) (string, error) {
	return p.fromOption(ctx, resource.WithHostID(), string(conventions.HostIDKey))
}

func (p systemMetadataProvider) OSDescription(ctx context.Context) (string, error) {
	return p.fromOption(ctx, resource.WithOSDescription(), string(conventions.OSDescriptionKey))
}

// OSName returns the OS name from host metadata.
func (systemMetadataProvider) OSName(ctx context.Context) (string, error) {
	info, err := host.InfoWithContext(ctx)
	if err != nil {
		return "", fmt.Errorf("OSName failed to get platform: %w", err)
	}
	return info.Platform, nil
}

// OSBuildID returns the OS build ID based on the platform.
func (systemMetadataProvider) OSBuildID(ctx context.Context) (string, error) {
	info, err := host.InfoWithContext(ctx)
	if err != nil {
		return "", fmt.Errorf("OSBuildID failed to get host info: %w", err)
	}
	switch runtime.GOOS {
	case "darwin":
		// macOS: read ProductBuildVersion from SystemVersion.plist
		data, err := os.ReadFile("/System/Library/CoreServices/SystemVersion.plist")
		if err != nil {
			return info.KernelVersion, nil
		}
		if v := parsePlistValue(data, "ProductBuildVersion"); v != "" {
			return v, nil
		}
		return info.KernelVersion, nil
	case "linux":
		// Linux: read BUILD_ID from /etc/os-release
		data, err := os.ReadFile("/etc/os-release")
		if err != nil {
			return info.KernelVersion, nil
		}
		if v := parseOSReleaseValue(data, "BUILD_ID"); v != "" {
			return v, nil
		}
		return info.KernelVersion, nil
	default:
		return info.KernelVersion, nil
	}
}

// parsePlistValue parses the XML plist and returns the <string> value after the given <key>.
func parsePlistValue(data []byte, key string) string {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	var currKey string
	for {
		tok, err := decoder.Token()
		if err != nil {
			break
		}
		if se, ok := tok.(xml.StartElement); ok {
			if se.Name.Local == "key" {
				var k string
				if err2 := decoder.DecodeElement(&k, &se); err2 == nil {
					currKey = k
				}
			} else if se.Name.Local == "string" && currKey == key {
				var v string
				if err2 := decoder.DecodeElement(&v, &se); err2 == nil {
					return v
				}
			}
		}
	}
	return ""
}

// parseOSReleaseValue parses key=value pairs from os-release data.
func parseOSReleaseValue(data []byte, key string) string {
	lines := strings.Split(string(data), "\n")
	prefix := key + "="
	for _, line := range lines {
		if strings.HasPrefix(line, prefix) {
			return strings.Trim(line[len(prefix):], `"`)
		}
	}
	return ""
}

func (systemMetadataProvider) HostArch() (string, error) {
	return internal.GOARCHtoHostArch(runtime.GOARCH), nil
}

func (systemMetadataProvider) HostIPs() (ips []net.IP, err error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	for _, iface := range ifaces {
		// skip if the interface is down or is a loopback interface
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		addrs, errAddr := iface.Addrs()
		if errAddr != nil {
			return nil, fmt.Errorf("failed to get addresses for interface %v: %w", iface, errAddr)
		}
		for _, addr := range addrs {
			ip, _, parseErr := net.ParseCIDR(addr.String())
			if parseErr != nil {
				return nil, fmt.Errorf("failed to parse address %q from interface %v: %w", addr, iface, parseErr)
			}

			if ip.IsLoopback() {
				// skip loopback IPs
				continue
			}

			ips = append(ips, ip)
		}
	}
	return ips, err
}

func (systemMetadataProvider) HostMACs() (macs []net.HardwareAddr, err error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	for _, iface := range ifaces {
		// skip if the interface is down or is a loopback interface
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		macs = append(macs, iface.HardwareAddr)
	}
	return macs, err
}

func (systemMetadataProvider) CPUInfo(ctx context.Context) ([]cpu.InfoStat, error) {
	return cpu.InfoWithContext(ctx)
}

func (systemMetadataProvider) HostInterfaces() ([]net.Interface, error) {
	return net.Interfaces()
}
