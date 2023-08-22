// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package consul // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/metadataproviders/consul"

import (
	"context"
	"fmt"

	"github.com/hashicorp/consul/api"
)

type Provider interface {
	Metadata(context.Context) (*Metadata, error)
}

type consulMetadataImpl struct {
	consulClient  *api.Client
	allowedLabels map[string]interface{}
}

type Metadata struct {
	NodeID       string
	Hostname     string
	Datacenter   string
	HostMetadata map[string]string
}

func NewProvider(client *api.Client, allowedLabels map[string]interface{}) Provider {
	return &consulMetadataImpl{consulClient: client, allowedLabels: allowedLabels}
}

func (d *consulMetadataImpl) Metadata(_ context.Context) (*Metadata, error) {
	var metadata Metadata
	self, err := d.consulClient.Agent().Self()
	if err != nil {
		return nil, fmt.Errorf("failed to get local agent information: %w", err)
	}

	config := self["Config"]
	if config == nil {
		return nil, fmt.Errorf("failed getting consul agent configuration. was 'Config' returned by consul?. resp: %+v", self)
	}

	hostname, ok := config["NodeName"].(string)
	if !ok {
		return nil, fmt.Errorf("failed getting consul hostname. was 'NodeName' returned by consul? resp: %+v", config)
	}
	metadata.Hostname = hostname

	datacenter, ok := config["Datacenter"].(string)
	if !ok {
		return nil, fmt.Errorf("failed getting consul datacenter. was 'Datacenter' returned by consul? resp: %+v", config)
	}
	metadata.Datacenter = datacenter

	nodeID, ok := config["NodeID"].(string)
	if !ok {
		return nil, fmt.Errorf("failed getting node ID. was 'NodeID' returned by consul? resp: %+v", config)
	}
	metadata.NodeID = nodeID

	meta := self["Meta"]
	if meta == nil {
		return &metadata, nil
	}

	metaMap := make(map[string]string)
	for k, v := range meta {
		if _, ok := d.allowedLabels[k]; ok {
			metaMap[k] = v.(string)
		}
	}
	metadata.HostMetadata = metaMap

	return &metadata, nil
}
