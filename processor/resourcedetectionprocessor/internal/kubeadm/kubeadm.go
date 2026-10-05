// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kubeadm // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/kubeadm"

import (
	"context"
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/processor"
	kubeadmdetector "go.opentelemetry.io/contrib/detectors/kubeadm"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/kubeadm/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/sdkbridge"
)

const (
	TypeStr = "kubeadm"
)

// makeClient is overridden in tests to substitute a fake Kubernetes client.
var makeClient = k8sconfig.MakeClient

var _ internal.Detector = (*detector)(nil)

// detector detects kubeadm cluster attributes. Detection is delegated to the upstream
// SDK detector so that the attributes reported here match the ones the collector's own
// telemetry reports.
type detector struct {
	detector              sdkresource.Detector
	logger                *zap.Logger
	resourceAttributes    metadata.ResourceAttributesConfig
	failOnMissingMetadata bool
}

func NewDetector(set processor.Settings, dcfg internal.DetectorConfig, failOnMissingMetadata bool) (internal.Detector, error) {
	cfg := dcfg.(Config)

	// The client is built here rather than by the SDK detector so that the configured
	// auth_type and kubeconfig keep applying.
	client, err := makeClient(cfg.APIConfig)
	if err != nil {
		return nil, fmt.Errorf("failed creating Kubernetes client: %w", err)
	}

	var sdkDetector sdkresource.Detector = kubeadmdetector.NewResourceDetector(kubeadmdetector.WithKubeClient(client))
	if failOnMissingMetadata {
		sdkDetector = strictDetector{sdkDetector}
	}

	return &detector{
		detector:              sdkDetector,
		logger:                set.Logger,
		resourceAttributes:    cfg.ResourceAttributes,
		failOnMissingMetadata: failOnMissingMetadata,
	}, nil
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
	// Detection runs unfiltered so that an empty result answers "is this a kubeadm
	// cluster?"; the configured attributes are applied afterwards. Unless
	// fail_on_missing_metadata is set, the bridge keeps a partial result.
	res, schemaURL, err := sdkbridge.Detect(ctx, d.detector)
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
