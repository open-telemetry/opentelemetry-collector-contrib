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

package consul // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/consul"

import (
	"context"
	"fmt"

	"github.com/hashicorp/consul/api"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/model/pdata"
	conventions "go.opentelemetry.io/collector/model/semconv/v1.6.1"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
)

const (
	// TypeStr is type of detector.
	TypeStr = "consul"
)

var _ internal.Detector = (*Detector)(nil)

// Detector is a system metadata detector
type Detector struct {
	provider consulMetadataCollector
	logger   *zap.Logger
}

// NewDetector creates a new system metadata detector
func NewDetector(p component.ProcessorCreateSettings, dcfg internal.DetectorConfig) (internal.Detector, error) {
	userCfg := dcfg.(Config)
	cfg := api.DefaultConfig()

	if userCfg.Address != "" {
		cfg.Address = userCfg.Address
	}
	if userCfg.Datacenter != "" {
		cfg.Datacenter = userCfg.Datacenter
	}
	if userCfg.Namespace != "" {
		cfg.Namespace = userCfg.Namespace
	}
	if userCfg.Token != "" {
		cfg.Token = userCfg.Token
	}
	if userCfg.TokenFile != "" {
		cfg.Token = userCfg.TokenFile
	}

	client, err := api.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed creating consul client: %w", err)
	}

	provider := newConsulMetadata(client, userCfg.MetaLabels)
	return &Detector{provider: provider, logger: p.Logger}, nil
}

// Detect detects system metadata and returns a resource with the available ones
func (d *Detector) Detect(ctx context.Context) (resource pdata.Resource, schemaURL string, err error) {
	res := pdata.NewResource()
	attrs := res.Attributes()

	metadata, err := d.provider.Metadata(ctx)
	if err != nil {
		return res, "", fmt.Errorf("failed to get consul metadata: %w", err)
	}

	attrs.InsertString(conventions.AttributeHostName, metadata.hostName)
	attrs.InsertString(conventions.AttributeCloudRegion, metadata.datacenter)
	attrs.InsertString(conventions.AttributeHostID, metadata.nodeID)

	for key, element := range metadata.hostMetadata {
		attrs.InsertString(key, element)
	}

	return res, conventions.SchemaURL, nil
}
