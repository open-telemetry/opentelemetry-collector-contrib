// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8snode // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/k8snode"

import (
	"context"
	"fmt"
	"os"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/processor"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/metadataproviders/k8snode"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/k8snode/internal/metadata"
)

const (
	TypeStr = "k8snode"
)

var _ internal.Detector = (*detector)(nil)

type detector struct {
	provider k8snode.Provider
	logger   *zap.Logger
	rb       *metadata.ResourceBuilder
}

func NewDetector(set processor.CreateSettings, dcfg internal.DetectorConfig) (internal.Detector, error) {
	cfg := dcfg.(Config)
	if err := cfg.UpdateDefaults(); err != nil {
		return nil, err
	}
	nodeName := os.Getenv(cfg.NodeFromEnvVar)
	k8snodeProvider, err := k8snode.NewProvider(nodeName, cfg.APIConfig)
	if err != nil {
		return nil, fmt.Errorf("failed creating k8snode detector: %w", err)
	}
	return &detector{
		provider: k8snodeProvider,
		logger:   set.Logger,
		rb:       metadata.NewResourceBuilder(cfg.ResourceAttributes),
	}, nil
}

func (d *detector) Detect(ctx context.Context) (resource pcommon.Resource, schemaURL string, err error) {
	nodeUID, err := d.provider.NodeUID(ctx)
	if err != nil {
		return pcommon.NewResource(), "", fmt.Errorf("failed getting k8s node UID: %w", err)
	}
	d.rb.SetK8sNodeUID(nodeUID)

	nodeName, err := d.provider.NodeName(ctx)
	if err != nil {
		return pcommon.NewResource(), "", fmt.Errorf("failed getting k8s node name: %w", err)
	}
	d.rb.SetK8sNodeName(nodeName)

	return d.rb.Emit(), conventions.SchemaURL, nil
}
