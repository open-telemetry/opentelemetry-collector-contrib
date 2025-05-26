// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package system // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/system"

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"

	"github.com/shirou/gopsutil/v4/cpu"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/processor"
	conventions "go.opentelemetry.io/otel/semconv/v1.6.1"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/metadataproviders/system"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/system/internal/metadata"
)

var _ = featuregate.GlobalRegistry().MustRegister(
	"processor.resourcedetection.hostCPUSteppingAsString",
	featuregate.StageStable,
	featuregate.WithRegisterDescription("Change type of host.cpu.stepping to string."),
	featuregate.WithRegisterFromVersion("v0.95.0"),
	featuregate.WithRegisterToVersion("v0.110.0"),
	featuregate.WithRegisterReferenceURL("https://github.com/open-telemetry/semantic-conventions/issues/664"),
)

const (
	// TypeStr is type of detector.
	TypeStr = "system"
)

var hostnameSourcesMap = map[string]func(*Detector) (string, error){
	"os":     getHostname,
	"dns":    getFQDN,
	"cname":  lookupCNAME,
	"lookup": reverseLookupHost,
}

var _ internal.Detector = (*Detector)(nil)

// Detector is a system metadata detector
type Detector struct {
	provider system.Provider
	logger   *zap.Logger
	cfg      Config
	rb       *metadata.ResourceBuilder
}

// NewDetector creates a new system metadata detector
func NewDetector(p processor.Settings, dcfg internal.DetectorConfig) (internal.Detector, error) {
	cfg := dcfg.(Config)
	if len(cfg.HostnameSources) == 0 {
		cfg.HostnameSources = []string{"dns", "os"}
	}

	return &Detector{
		provider: system.NewProvider(),
		logger:   p.Logger,
		cfg:      cfg,
		rb:       metadata.NewResourceBuilder(cfg.ResourceAttributes),
	}, nil
}

// toIEEERA converts a MAC address to IEEE RA format.
// Per the spec: "MAC Addresses MUST be represented in IEEE RA hexadecimal form: as hyphen-separated
// octets in uppercase hexadecimal form from most to least significant."
// Golang returns MAC addresses as colon-separated octets in lowercase hexadecimal form from most
// to least significant, so we need to:
// - Replace colons with hyphens
// - Convert to uppercase
func toIEEERA(mac net.HardwareAddr) string {
	return strings.ToUpper(strings.ReplaceAll(mac.String(), ":", "-"))
}

// Detect detects system metadata and returns a resource with the available ones
func (d *Detector) Detect(ctx context.Context) (resource pcommon.Resource, schemaURL string, err error) {
	var hostname string

	osType, err := d.provider.OSType()
	if err != nil {
		return pcommon.NewResource(), "", fmt.Errorf("failed getting OS type: %w", err)
	}

	osVersion, err := d.provider.OSVersion()
	if err != nil {
		return pcommon.NewResource(), "", fmt.Errorf("failed getting OS version: %w", err)
	}

	hostArch, err := d.provider.HostArch()
	if err != nil {
		return pcommon.NewResource(), "", fmt.Errorf("failed getting host architecture: %w", err)
	}

	var hostIPAttribute []any
	if d.cfg.ResourceAttributes.HostIP.Enabled {
		hostIPs, errIPs := d.provider.HostIPs()
		if errIPs != nil {
			return pcommon.NewResource(), "", fmt.Errorf("failed getting host IP addresses: %w", errIPs)
		}
		for _, ip := range hostIPs {
			hostIPAttribute = append(hostIPAttribute, ip.String())
		}
	}

	var hostMACAttribute []any
	if d.cfg.ResourceAttributes.HostMac.Enabled {
		hostMACs, errMACs := d.provider.HostMACs()
		if errMACs != nil {
			return pcommon.NewResource(), "", fmt.Errorf("failed to get host MAC addresses: %w", errMACs)
		}
		for _, mac := range hostMACs {
			hostMACAttribute = append(hostMACAttribute, toIEEERA(mac))
		}
	}

	osDescription, err := d.provider.OSDescription(ctx)
	if err != nil {
		return pcommon.NewResource(), "", fmt.Errorf("failed getting OS description: %w", err)
	}

	var cpuInfo []cpu.InfoStat
	if d.cfg.ResourceAttributes.HostCPUCacheL2Size.Enabled || d.cfg.ResourceAttributes.HostCPUFamily.Enabled ||
		d.cfg.ResourceAttributes.HostCPUModelID.Enabled || d.cfg.ResourceAttributes.HostCPUVendorID.Enabled ||
		d.cfg.ResourceAttributes.HostCPUModelName.Enabled || d.cfg.ResourceAttributes.HostCPUStepping.Enabled {
		cpuInfo, err = d.provider.CPUInfo(ctx)
		if err != nil {
			return pcommon.NewResource(), "", fmt.Errorf("failed getting host cpuinfo: %w", err)
		}
	}

	for _, source := range d.cfg.HostnameSources {
		getHostFromSource := hostnameSourcesMap[source]
		hostname, err = getHostFromSource(d)
		if err == nil {
			d.rb.SetHostName(hostname)
			d.rb.SetOsType(osType)
			d.rb.SetOsVersion(osVersion)
			if d.cfg.ResourceAttributes.HostID.Enabled {
				if hostID, hostIDErr := d.provider.HostID(ctx); hostIDErr == nil {
					d.rb.SetHostID(hostID)
				} else {
					d.logger.Warn("failed to get host ID", zap.Error(hostIDErr))
				}
			}
			d.rb.SetHostArch(hostArch)
			d.rb.SetHostIP(hostIPAttribute)
			d.rb.SetHostMac(hostMACAttribute)
			d.rb.SetOsDescription(osDescription)
			if len(cpuInfo) > 0 {
				setHostCPUInfo(d, cpuInfo[0])
			}
			return d.rb.Emit(), conventions.SchemaURL, nil
		}
		d.logger.Debug(err.Error())
	}

	return pcommon.NewResource(), "", errors.New("all hostname sources failed to get hostname")
}

// getHostname returns OS hostname
func getHostname(d *Detector) (string, error) {
	hostname, err := d.provider.Hostname()
	if err != nil {
		return "", fmt.Errorf("failed getting OS hostname: %w", err)
	}
	return hostname, nil
}

// getFQDN returns FQDN of the host
func getFQDN(d *Detector) (string, error) {
	hostname, err := d.provider.FQDN()
	if err != nil {
		return "", fmt.Errorf("getFQDN failed getting FQDN: %w", err)
	}
	return hostname, nil
}

func lookupCNAME(d *Detector) (string, error) {
	cname, err := d.provider.LookupCNAME()
	if err != nil {
		return "", fmt.Errorf("lookupCNAME failed to get CNAME: %w", err)
	}
	return cname, nil
}

func reverseLookupHost(d *Detector) (string, error) {
	hostname, err := d.provider.ReverseLookupHost()
	if err != nil {
		return "", fmt.Errorf("reverseLookupHost failed to lookup host: %w", err)
	}
	return hostname, nil
}

func setHostCPUInfo(d *Detector, cpuInfo cpu.InfoStat) {
	d.logger.Debug("getting host's cpuinfo", zap.String("coreID", cpuInfo.CoreID))
	d.rb.SetHostCPUVendorID(cpuInfo.VendorID)
	d.rb.SetHostCPUFamily(cpuInfo.Family)

	// For windows, this field is left blank. See https://github.com/shirou/gopsutil/blob/v3.23.9/cpu/cpu_windows.go#L113
	// Skip setting modelId if the field is blank.
	// ISSUE: https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/27675
	if cpuInfo.Model != "" {
		d.rb.SetHostCPUModelID(cpuInfo.Model)
	}

	d.rb.SetHostCPUModelName(cpuInfo.ModelName)
	d.rb.SetHostCPUStepping(fmt.Sprintf("%d", cpuInfo.Stepping))
	d.rb.SetHostCPUCacheL2Size(int64(cpuInfo.CacheSize))
}
