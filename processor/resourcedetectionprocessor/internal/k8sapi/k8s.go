// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build (!remove_all_resourcedetection_detectors || enable_detector_k8s_api) && !omit_detector_k8s_api

package k8sapi // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/k8sapi"

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/processor"
	k8sapidetector "go.opentelemetry.io/contrib/detectors/k8sapi"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
	"go.uber.org/zap"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/k8sapi/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/sdkbridge"
)

var _ internal.Detector = (*detector)(nil)

// newClient is overridden in tests to substitute the Kubernetes client.
var newClient = k8sconfig.MakeClient

type detector struct {
	cfg Config
	// mu serializes Detect, which lazily creates sdkDetector.
	mu sync.Mutex
	// sdkDetector is created on first use by initDetector, so that the detector can be
	// built outside Kubernetes.
	sdkDetector           sdkresource.Detector
	logger                *zap.Logger
	ra                    *metadata.ResourceAttributesConfig
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
		failOnMissingMetadata: failOnMissingMetadata,
	}, nil
}

// errMetadataUnavailable marks initDetector failures caused by the environment rather
// than the configuration: no Kubernetes pod, or no node name to look up.
var errMetadataUnavailable = errors.New("k8s node metadata unavailable")

// initDetector creates the go-contrib k8sapi detector if it doesn't exist yet. A failure
// is not cached, so the next Detect call tries again. The caller holds d.mu.
func (d *detector) initDetector() error {
	if d.sdkDetector != nil {
		return nil
	}
	if err := internal.CheckInClusterEnv(d.cfg.APIConfig); err != nil {
		return fmt.Errorf("%w: %w", errMetadataUnavailable, err)
	}
	if err := d.cfg.UpdateDefaults(); err != nil {
		return fmt.Errorf("%w: %w", errMetadataUnavailable, err)
	}
	client, err := newClient(k8sconfig.FromConfigTypes(d.cfg.APIConfig))
	if err != nil {
		return fmt.Errorf("failed creating k8snode detector: failed to create K8s API client: %w", err)
	}
	opts := []k8sapidetector.Option{
		k8sapidetector.WithKubeClient(client),
		k8sapidetector.WithNodeEnvVar(d.cfg.NodeFromEnvVar),
	}
	if d.ra.HostID.Enabled {
		opts = append(opts, k8sapidetector.WithHostID())
	}
	d.sdkDetector = k8sapidetector.NewResourceDetector(opts...)
	return nil
}

// NewDeprecatedDetector logs a deprecation warning and then delegates to NewDetector.
// Use this for the k8snode alias so users are notified to migrate to k8s_api.
func NewDeprecatedDetector(set processor.Settings, dcfg internal.DetectorConfig, failOnMissingMetadata bool) (internal.Detector, error) {
	set.Logger.Warn("the k8snode detector name is deprecated, use k8s_api instead")
	return NewDetector(set, dcfg, failOnMissingMetadata)
}

// errRecorder keeps the error that sdkbridge.Detect drops for partial resources, so that
// Detect can tell which lookup failed and why.
type errRecorder struct {
	sdkresource.Detector
	err error
}

func (r *errRecorder) Detect(ctx context.Context) (*sdkresource.Resource, error) {
	res, err := r.Detector.Detect(ctx)
	r.err = err
	return res, err
}

func (d *detector) Detect(ctx context.Context) (pcommon.Resource, string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if err := d.initDetector(); err != nil {
		// Outside Kubernetes, or without a node name, there is nothing to detect. Any other
		// failure to create the client is reported whatever fail_on_missing_metadata says,
		// as NewDetector used to.
		if !errors.Is(err, errMetadataUnavailable) || d.failOnMissingMetadata {
			return pcommon.NewResource(), "", err
		}
		d.logger.Debug("k8snode metadata unavailable", zap.Error(err))
		return pcommon.NewResource(), "", nil
	}

	rec := &errRecorder{Detector: d.sdkDetector}
	res, schemaURL, err := sdkbridge.Detect(ctx, rec)
	if err != nil {
		return pcommon.NewResource(), "", err
	}

	// The node lookup yields k8s.node.uid, k8s.node.name and host.id together, so a missing
	// k8s.node.uid means the lookup failed.
	if d.ra.K8sNodeUID.Enabled || d.ra.K8sNodeName.Enabled || d.ra.HostID.Enabled {
		if _, ok := res.Attributes().Get("k8s.node.uid"); !ok {
			err := fmt.Errorf("failed getting k8s node: %w", rec.err)
			if d.failOnMissingMetadata {
				return pcommon.NewResource(), "", err
			}
			d.logger.Debug("k8snode metadata unavailable", zap.Error(err))
			return pcommon.NewResource(), "", nil
		}
	}

	if d.ra.K8sClusterUID.Enabled && rec.err != nil {
		if _, ok := res.Attributes().Get("k8s.cluster.uid"); !ok {
			if !k8serrors.IsForbidden(rec.err) && !k8serrors.IsUnauthorized(rec.err) {
				return pcommon.NewResource(), "", fmt.Errorf("failed getting k8s cluster UID: %w", rec.err)
			}
			// Warn and skip for backward compatibility: existing deployments may lack kube-system RBAC.
			// This will become a hard error in a future release.
			d.logger.Warn("no permission to get kube-system namespace, skipping k8s.cluster.uid; this will become an error in a future release — disable the attribute or grant 'get' on namespaces/kube-system", zap.Error(rec.err))
		}
	}

	sdkbridge.RemoveDisabledAttributes(res, d.ra)
	return res, schemaURL, nil
}
