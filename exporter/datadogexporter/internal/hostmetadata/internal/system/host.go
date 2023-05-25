// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package system contains the system hostname provider
package system // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/hostmetadata/internal/system"

import (
	"context"
	"os"
	"sync"

	"github.com/DataDog/opentelemetry-mapping-go/pkg/otlp/attributes/source"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/hostmetadata/valid"
)

type HostInfo struct {
	OS   string
	FQDN string
}

// GetHostInfo gets system information about the hostname
func GetHostInfo(logger *zap.Logger) (hostInfo *HostInfo) {
	hostInfo = &HostInfo{}

	if hostname, err := getSystemFQDN(); err == nil {
		hostInfo.FQDN = hostname
	} else {
		logger.Warn("Could not get FQDN Hostname", zap.Error(err))
	}

	if hostname, err := os.Hostname(); err == nil {
		hostInfo.OS = hostname
	} else {
		logger.Warn("Could not get OS Hostname", zap.Error(err))
	}

	return
}

// GetHostname gets the hostname provided by the system
func (hi *HostInfo) GetHostname(logger *zap.Logger) string {
	if hi.FQDN == "" {
		// Don't report failure since FQDN was just not available
		return hi.OS
	} else if err := valid.Hostname(hi.FQDN); err != nil {
		logger.Info("FQDN is not valid", zap.Error(err))
		return hi.OS
	}

	return hi.FQDN
}

var _ source.Provider = (*Provider)(nil)

type Provider struct {
	once     sync.Once
	hostInfo HostInfo

	logger *zap.Logger
}

func (p *Provider) fillHostInfo() {
	p.once.Do(func() { p.hostInfo = *GetHostInfo(p.logger) })
}

func (p *Provider) Source(context.Context) (source.Source, error) {
	p.fillHostInfo()
	return source.Source{Kind: source.HostnameKind, Identifier: p.hostInfo.GetHostname(p.logger)}, nil
}

func (p *Provider) HostInfo() *HostInfo {
	p.fillHostInfo()
	return &p.hostInfo
}

func NewProvider(logger *zap.Logger) *Provider {
	return &Provider{logger: logger}
}
