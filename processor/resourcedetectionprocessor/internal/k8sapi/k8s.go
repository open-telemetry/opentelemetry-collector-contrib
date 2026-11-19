// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build (!remove_all_resourcedetection_detectors || enable_detector_k8s_api) && !omit_detector_k8s_api

package k8sapi // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/k8sapi"

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/processor"
	conventions "go.opentelemetry.io/otel/semconv/v1.43.0"
	"go.uber.org/zap"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/metadataproviders/k8snode"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/k8sapi/internal/metadata"
)

var _ internal.Detector = (*detector)(nil)

// newProvider is overridden in tests to substitute the k8snode provider.
var newProvider = k8snode.NewProvider

type detector struct {
	cfg Config
	// mu serializes Detect, which lazily creates provider and mutates rb.
	mu sync.Mutex
	// provider is created on first use by initProvider, so that the detector can be
	// built outside Kubernetes.
	provider              k8snode.Provider
	logger                *zap.Logger
	ra                    *metadata.ResourceAttributesConfig
	rb                    *metadata.ResourceBuilder
	failOnMissingMetadata bool
}

// NewDetector returns a detector for Kubernetes node metadata. The node name and the
// Kubernetes client are resolved on the first Detect call, so the detector can be built
// outside Kubernetes.
func NewDetector(set processor.Settings, dcfg internal.DetectorConfig, failOnMissingMetadata bool) (internal.Detector, error) {
	cfg := dcfg.(Config)
	if err := cfg.APIConfig.Validate(); err != nil {
		return nil, err
	}
	return &detector{
		cfg:                   cfg,
		logger:                set.Logger,
		ra:                    &cfg.ResourceAttributes,
		rb:                    metadata.NewResourceBuilder(cfg.ResourceAttributes),
		failOnMissingMetadata: failOnMissingMetadata,
	}, nil
}

// errMetadataUnavailable marks initProvider failures caused by the environment rather
// than the configuration: no Kubernetes pod, or no node name to look up.
var errMetadataUnavailable = errors.New("k8s node metadata unavailable")

// initProvider creates the k8snode provider if it doesn't exist yet. A failure is not
// cached, so the next Detect call tries again. The caller holds d.mu.
func (d *detector) initProvider() error {
	if d.provider != nil {
		return nil
	}
	if err := internal.CheckInClusterEnv(d.cfg.APIConfig); err != nil {
		return fmt.Errorf("%w: %w", errMetadataUnavailable, err)
	}
	if err := d.cfg.UpdateDefaults(); err != nil {
		return fmt.Errorf("%w: %w", errMetadataUnavailable, err)
	}
	provider, err := newProvider(os.Getenv(d.cfg.NodeFromEnvVar), k8sconfig.FromConfigTypes(d.cfg.APIConfig))
	if err != nil {
		return fmt.Errorf("failed creating k8snode detector: %w", err)
	}
	d.provider = provider
	return nil
}

// NewDeprecatedDetector logs a deprecation warning and then delegates to NewDetector.
// Use this for the k8snode alias so users are notified to migrate to k8s_api.
func NewDeprecatedDetector(set processor.Settings, dcfg internal.DetectorConfig, failOnMissingMetadata bool) (internal.Detector, error) {
	set.Logger.Warn("the k8snode detector name is deprecated, use k8s_api instead")
	return NewDetector(set, dcfg, failOnMissingMetadata)
}

func (d *detector) Detect(ctx context.Context) (resource pcommon.Resource, schemaURL string, err error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if err := d.initProvider(); err != nil {
		// Outside Kubernetes, or without a node name, there is nothing to detect. Any other
		// failure to create the client is reported whatever fail_on_missing_metadata says,
		// as NewDetector used to.
		if !errors.Is(err, errMetadataUnavailable) || d.failOnMissingMetadata {
			return pcommon.NewResource(), "", err
		}
		d.logger.Debug("k8snode metadata unavailable", zap.Error(err))
		return pcommon.NewResource(), "", nil
	}

	if d.ra.K8sNodeUID.Enabled {
		nodeUID, err := d.provider.NodeUID(ctx)
		if err != nil {
			if d.failOnMissingMetadata {
				return pcommon.NewResource(), "", fmt.Errorf("failed getting k8s node UID: %w", err)
			}
			d.logger.Debug("k8snode metadata unavailable", zap.Error(err))
			return pcommon.NewResource(), "", nil
		}
		d.rb.SetK8sNodeUID(nodeUID)
	}

	if d.ra.K8sNodeName.Enabled {
		nodeName, err := d.provider.NodeName(ctx)
		if err != nil {
			if d.failOnMissingMetadata {
				return pcommon.NewResource(), "", fmt.Errorf("failed getting k8s node name: %w", err)
			}
			d.logger.Debug("k8snode metadata unavailable", zap.Error(err))
			return pcommon.NewResource(), "", nil
		}
		d.rb.SetK8sNodeName(nodeName)
	}

	if d.ra.HostID.Enabled {
		machineID, err := d.provider.NodeMachineID(ctx)
		if err != nil {
			if d.failOnMissingMetadata {
				return pcommon.NewResource(), "", fmt.Errorf("failed getting k8s node machine ID: %w", err)
			}
			d.logger.Debug("k8snode metadata unavailable", zap.Error(err))
			return pcommon.NewResource(), "", nil
		}
		// Some kubelets don't report a machine ID; omit host.id rather than emit it empty.
		if machineID != "" {
			d.rb.SetHostID(machineID)
		} else {
			d.logger.Debug("k8s node reports an empty machine ID, skipping host.id")
		}
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
