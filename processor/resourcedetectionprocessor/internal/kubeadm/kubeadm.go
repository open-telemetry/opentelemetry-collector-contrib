// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build (!remove_all_resourcedetection_detectors || enable_detector_kubeadm) && !omit_detector_kubeadm

package kubeadm // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/kubeadm"

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/processor"
	kubeadmdetector "go.opentelemetry.io/contrib/detectors/kubeadm"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig/k8sconfigtypes"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/kubeadm/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/sdkbridge"
)

// makeClient is overridden in tests to substitute a fake Kubernetes client.
var makeClient = k8sconfig.MakeClient

var _ internal.Detector = (*detector)(nil)

// detector detects kubeadm cluster attributes. Detection is delegated to the upstream
// SDK detector so that the attributes reported here match the ones the collector's own
// telemetry reports.
type detector struct {
	apiConfig k8sconfigtypes.APIConfig
	// mu guards detector, which is created on first use by getDetector so that the
	// detector can be built outside Kubernetes.
	mu                    sync.Mutex
	detector              sdkresource.Detector
	logger                *zap.Logger
	resourceAttributes    metadata.ResourceAttributesConfig
	failOnMissingMetadata bool
}

// NewDetector returns a kubeadm detector. The Kubernetes client is created on the
// first Detect call, so the detector can be built outside Kubernetes.
func NewDetector(set processor.Settings, dcfg internal.DetectorConfig, failOnMissingMetadata bool) (internal.Detector, error) {
	cfg := dcfg.(Config)
	if err := cfg.APIConfig.Validate(); err != nil {
		return nil, err
	}

	return &detector{
		apiConfig:             cfg.APIConfig,
		logger:                set.Logger,
		resourceAttributes:    cfg.ResourceAttributes,
		failOnMissingMetadata: failOnMissingMetadata,
	}, nil
}

// getDetector returns the SDK detector, creating it and its Kubernetes client if they
// don't exist yet. A failure is not cached, so the next Detect call tries again.
func (d *detector) getDetector() (sdkresource.Detector, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.detector != nil {
		return d.detector, nil
	}

	if err := internal.CheckInClusterEnv(d.apiConfig); err != nil {
		return nil, err
	}

	// The client is built here rather than by the SDK detector so that the configured
	// auth_type and kubeconfig keep applying.
	client, err := makeClient(k8sconfig.FromConfigTypes(d.apiConfig))
	if err != nil {
		return nil, fmt.Errorf("failed creating Kubernetes client: %w", err)
	}

	var sdkDetector sdkresource.Detector = kubeadmdetector.NewResourceDetector(kubeadmdetector.WithKubeClient(client))
	if d.failOnMissingMetadata {
		sdkDetector = strictDetector{sdkDetector}
	}
	d.detector = sdkDetector
	return sdkDetector, nil
}

// strictDetector reports a partial result as a failure. Each kubeadm attribute comes
// from its own API call, so a partial result means a call failed, which is what
// fail_on_missing_metadata asks to surface so that the processor retries.
type strictDetector struct {
	sdkresource.Detector
}

func (s strictDetector) Detect(ctx context.Context) (*sdkresource.Resource, error) {
	res, err := s.Detector.Detect(ctx)
	if errors.Is(err, sdkresource.ErrPartialResource) {
		// %v rather than %w: sdkbridge keeps any result whose error wraps ErrPartialResource.
		return nil, fmt.Errorf("kubeadm metadata incomplete: %v", err) //nolint:errorlint
	}
	return res, err
}

func (d *detector) Detect(ctx context.Context) (pcommon.Resource, string, error) {
	sdkDetector, err := d.getDetector()
	if err != nil {
		// Outside Kubernetes there is nothing to detect. Any other failure to create the
		// client is reported whatever fail_on_missing_metadata says, as NewDetector used to.
		if !errors.Is(err, internal.ErrNotInKubernetes) || d.failOnMissingMetadata {
			return pcommon.NewResource(), "", err
		}
		d.logger.Debug("kubeadm metadata unavailable", zap.Error(err))
		return pcommon.NewResource(), "", nil
	}

	// Detection runs unfiltered so that an empty result answers "is this a kubeadm
	// cluster?"; the configured attributes are applied afterwards. Unless
	// fail_on_missing_metadata is set, the bridge keeps a partial result.
	res, schemaURL, err := sdkbridge.Detect(ctx, sdkDetector)
	if err != nil {
		d.logger.Debug("kubeadm metadata unavailable", zap.Error(err))
		if d.failOnMissingMetadata {
			return pcommon.NewResource(), "", err
		}
		return pcommon.NewResource(), "", nil
	}

	// The SDK detector reports an empty resource and no error when the kubeadm-config
	// ConfigMap does not exist, i.e. the cluster was not provisioned by kubeadm.
	if res.Attributes().Len() == 0 {
		d.logger.Debug("kubeadm detector: metadata unavailable or not a kubeadm cluster")
		if d.failOnMissingMetadata {
			return pcommon.NewResource(), "", errors.New("kubeadm metadata unavailable")
		}
		return pcommon.NewResource(), "", nil
	}

	sdkbridge.RemoveDisabledAttributes(res, d.resourceAttributes)
	return res, schemaURL, nil
}
