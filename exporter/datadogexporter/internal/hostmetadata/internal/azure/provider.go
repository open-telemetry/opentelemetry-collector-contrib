// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package azure contains the Azure hostname provider
package azure // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/hostmetadata/internal/azure"

import (
	"context"
	"fmt"
	"strings"

	"github.com/DataDog/opentelemetry-mapping-go/pkg/otlp/attributes/source"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/hostmetadata/provider"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/metadataproviders/azure"
)

var _ source.Provider = (*Provider)(nil)
var _ provider.ClusterNameProvider = (*Provider)(nil)

type Provider struct {
	detector azure.Provider
}

// Hostname returns the Azure cloud integration hostname.
func (p *Provider) Source(ctx context.Context) (source.Source, error) {
	metadata, err := p.detector.Metadata(ctx)
	if err != nil {
		return source.Source{}, err
	}

	return source.Source{Kind: source.HostnameKind, Identifier: metadata.VMID}, nil
}

// ClusterName gets the AKS cluster name from the resource group name.
func (p *Provider) ClusterName(ctx context.Context) (string, error) {
	metadata, err := p.detector.Metadata(ctx)
	if err != nil {
		return "", err
	}

	// Code comes from https://github.com/DataDog/datadog-agent/blob/1b4afdd6a/pkg/util/cloudproviders/azure/azure.go#L72
	// It expects the resource group name to have the format (MC|mc)_resource-group_cluster-name_zone.
	splitAll := strings.Split(metadata.ResourceGroupName, "_")
	if len(splitAll) < 4 || strings.ToLower(splitAll[0]) != "mc" {
		return "", fmt.Errorf("cannot parse the clustername from resource group name: %s", metadata.ResourceGroupName)
	}

	return splitAll[len(splitAll)-2], nil
}

// NewProvider creates a new Azure hostname provider.
func NewProvider() *Provider {
	return &Provider{detector: azure.NewProvider()}
}
