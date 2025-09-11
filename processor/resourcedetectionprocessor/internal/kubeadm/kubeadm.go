// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kubeadm // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/kubeadm"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/processor"
	conventions "go.opentelemetry.io/otel/semconv/v1.6.1"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/metadataproviders/kubeadm"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/kubeadm/internal/metadata"
)

const (
	TypeStr                    = "kubeadm"
	defaultConfigMapName       = "kubeadm-config"
	defaultKubeSystemNamespace = "kube-system"
)

var _ internal.Detector = (*detector)(nil)

type detector struct {
	provider kubeadm.Provider
	logger   *zap.Logger
	ra       *metadata.ResourceAttributesConfig
	rb       *metadata.ResourceBuilder
}

func NewDetector(set processor.Settings, dcfg internal.DetectorConfig) (internal.Detector, error) {
	cfg := dcfg.(Config)

	kubeadmProvider, err := kubeadm.NewProvider(defaultConfigMapName, defaultKubeSystemNamespace, cfg.APIConfig)
	if err != nil {
		return nil, fmt.Errorf("failed creating kubeadm provider: %w", err)
	}

	return &detector{
		provider: kubeadmProvider,
		logger:   set.Logger,
		ra:       &cfg.ResourceAttributes,
		rb:       metadata.NewResourceBuilder(cfg.ResourceAttributes),
	}, nil
}

func (d *detector) Detect(ctx context.Context) (resource pcommon.Resource, schemaURL string, err error) {
	if d.ra.K8sClusterName.Enabled {
		clusterName, err := d.provider.ClusterName(ctx)
		if err != nil {
			return pcommon.NewResource(), "", fmt.Errorf("failed getting k8s cluster name: %w", err)
		}
		d.rb.SetK8sClusterName(clusterName)
	}

	if d.ra.K8sClusterUID.Enabled {
		clusterUID, err := d.provider.ClusterUID(ctx)
		if err != nil {
			return pcommon.NewResource(), "", fmt.Errorf("failed getting k8s cluster uid: %w", err)
		}
		d.rb.SetK8sClusterUID(clusterUID)
	}

	return d.rb.Emit(), conventions.SchemaURL, nil
}
