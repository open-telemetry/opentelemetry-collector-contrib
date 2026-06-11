// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sapi // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/k8sapi"

import (
	"context"
	"fmt"
	"os"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/processor"
	conventions "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.uber.org/zap"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/metadataproviders/k8snode"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/k8sapi/internal/metadata"
)

const (
	TypeStr      = "k8s_api"
	TypeStrAlias = "k8snode" // Deprecated: use TypeStr
)

var _ internal.Detector = (*detector)(nil)

type detector struct {
	provider k8snode.Provider
	logger   *zap.Logger
	ra       *metadata.ResourceAttributesConfig
	rb       *metadata.ResourceBuilder
}

func NewDetector(set processor.Settings, dcfg internal.DetectorConfig) (internal.Detector, error) {
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
		ra:       &cfg.ResourceAttributes,
		rb:       metadata.NewResourceBuilder(cfg.ResourceAttributes),
	}, nil
}

// NewDeprecatedDetector logs a deprecation warning and then delegates to NewDetector.
// Use this for the k8snode alias so users are notified to migrate to k8s_api.
func NewDeprecatedDetector(set processor.Settings, dcfg internal.DetectorConfig) (internal.Detector, error) {
	set.Logger.Warn("the k8snode detector name is deprecated, use k8s_api instead")
	return NewDetector(set, dcfg)
}

func (d *detector) Detect(ctx context.Context) (resource pcommon.Resource, schemaURL string, err error) {
	if d.ra.K8sNodeUID.Enabled {
		nodeUID, err := d.provider.NodeUID(ctx)
		if err != nil {
			return pcommon.NewResource(), "", fmt.Errorf("failed getting k8s node UID: %w", err)
		}
		d.rb.SetK8sNodeUID(nodeUID)
	}

	if d.ra.K8sNodeName.Enabled {
		nodeName, err := d.provider.NodeName(ctx)
		if err != nil {
			return pcommon.NewResource(), "", fmt.Errorf("failed getting k8s node name: %w", err)
		}
		d.rb.SetK8sNodeName(nodeName)
	}

	if d.ra.K8sClusterUID.Enabled {
		clusterUID, err := d.provider.ClusterUID(ctx)
		if err != nil {
			if !k8serrors.IsForbidden(err) && !k8serrors.IsUnauthorized(err) {
				return pcommon.NewResource(), "", fmt.Errorf("failed getting k8s cluster UID: %w", err)
			}
			// Warn and skip for backward compatibility: existing deployments may lack kube-system RBAC.
			// This will become a hard error in a future release.
			d.logger.Warn("no permission to get kube-system namespace, skipping k8s.cluster.uid; this will become an error in a future release — disable the attribute or grant 'get' on namespaces/kube-system", zap.Error(err))
		} else {
			d.rb.SetK8sClusterUID(clusterUID)
		}
	}

	return d.rb.Emit(), conventions.SchemaURL, nil
}
