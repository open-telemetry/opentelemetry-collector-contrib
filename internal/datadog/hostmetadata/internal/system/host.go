// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package system contains the system hostname provider
package system // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/datadog/hostmetadata/internal/system"

import (
	"context"
	"os"
	"sync"

	"github.com/DataDog/datadog-agent/pkg/util/hostname/validate"
	"github.com/DataDog/opentelemetry-mapping-go/pkg/otlp/attributes/source"
	"go.uber.org/zap"
)

type HostInfo struct {
	OS   string
	FQDN string
}

// GetHostInfo gets system information about the hostname
func GetHostInfo(ctx context.Context, logger *zap.Logger) (hostInfo *HostInfo) {
	hostInfo = &HostInfo{}

	if hostname, err := getSystemFQDN(ctx); err == nil {
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
	} else if err := validate.ValidHostname(hi.FQDN); err != nil {
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

func (p *Provider) fillHostInfo(ctx context.Context) {
	p.once.Do(func() { p.hostInfo = *GetHostInfo(ctx, p.logger) })
}

func (p *Provider) Source(ctx context.Context) (source.Source, error) {
	p.fillHostInfo(ctx)
	return source.Source{Kind: source.HostnameKind, Identifier: p.hostInfo.GetHostname(p.logger)}, nil
}

func (p *Provider) HostInfo(ctx context.Context) *HostInfo {
	p.fillHostInfo(ctx)
	return &p.hostInfo
}

func NewProvider(logger *zap.Logger) *Provider {
	return &Provider{logger: logger}
}
