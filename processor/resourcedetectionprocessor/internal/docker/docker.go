// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build (!remove_all_resourcedetection_detectors || enable_detector_docker) && !omit_detector_docker

package docker // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/docker"

import (
	"context"
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/processor"
	dockerdetector "go.opentelemetry.io/contrib/detectors/docker"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
	conventions "go.opentelemetry.io/otel/semconv/v1.43.0"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/docker/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/sdkbridge"
)

var _ internal.Detector = (*Detector)(nil)

// newResourceDetector is overridden in tests to substitute a fake SDK detector.
var newResourceDetector = func() sdkresource.Detector {
	return dockerdetector.NewResourceDetector()
}

// Detector is a system metadata detector
type Detector struct {
	detector              sdkresource.Detector
	logger                *zap.Logger
	resourceAttributes    metadata.ResourceAttributesConfig
	failOnMissingMetadata bool
}

// NewDetector creates a new system metadata detector
func NewDetector(p processor.Settings, cfg internal.DetectorConfig, failOnMissingMetadata bool) (internal.Detector, error) {
	resourceAttributes := cfg.(Config).ResourceAttributes
	sdkDetector := newResourceDetector()
	if failOnMissingMetadata && (resourceAttributes.HostName.Enabled || resourceAttributes.OsType.Enabled) {
		sdkDetector = strictDetector{sdkDetector}
	}
	return &Detector{
		detector:              sdkDetector,
		logger:                p.Logger,
		resourceAttributes:    resourceAttributes,
		failOnMissingMetadata: failOnMissingMetadata,
	}, nil
}

// strictDetector reports a partial result as a failure. The SDK detector only returns a
// partial result when the daemon's Info call fails, and that call supplies host.name and
// os.type, so it is used only when one of those is enabled.
type strictDetector struct {
	sdkresource.Detector
}

func (s strictDetector) Detect(ctx context.Context) (*sdkresource.Resource, error) {
	res, err := s.Detector.Detect(ctx)
	if errors.Is(err, sdkresource.ErrPartialResource) {
		// %v rather than %w: sdkbridge keeps any result whose error wraps ErrPartialResource.
		return nil, fmt.Errorf("docker metadata incomplete: %v", err) //nolint:errorlint
	}
	return res, err
}

// Detect detects system metadata and returns a resource with the available ones
func (d *Detector) Detect(ctx context.Context) (pcommon.Resource, string, error) {
	// Detection runs unfiltered so that an empty result answers "is this process running in a
	// Docker container?"; the configured attributes are applied afterwards. Unless
	// fail_on_missing_metadata is set and host.name or os.type is enabled, the bridge keeps a
	// partial result.
	res, schemaURL, err := sdkbridge.Detect(ctx, d.detector)
	if err != nil {
		d.logger.Debug("docker metadata unavailable", zap.Error(err))
		if d.failOnMissingMetadata {
			return pcommon.NewResource(), "", err
		}
		return pcommon.NewResource(), "", nil
	}

	// The SDK detector returns an empty resource both when the daemon is unreachable and when
	// no container matches this process's container ID or hostname.
	if res.Attributes().Len() == 0 {
		d.logger.Debug("docker detector: daemon unreachable or not running in a container")
		if d.failOnMissingMetadata {
			return pcommon.NewResource(), "", errors.New("docker metadata unavailable")
		}
		return pcommon.NewResource(), "", nil
	}

	// Restore the shapes this detector reported before detection moved to the SDK detector.
	// The rewrite runs on the unfiltered resource because container.image.id, which carries
	// the value container.image.name used to hold, is disabled by default.
	if !metadata.ProcessorResourcedetectionDockerEmitSemconvContainerAttributesFeatureGate.IsEnabled() {
		attrs := res.Attributes()
		if name, ok := attrs.Get(string(conventions.ContainerNameKey)); ok {
			attrs.PutStr(string(conventions.ContainerNameKey), "/"+name.Str())
		}
		if id, ok := attrs.Get(string(conventions.ContainerImageIDKey)); ok {
			attrs.PutStr(string(conventions.ContainerImageNameKey), id.Str())
		}
	}

	sdkbridge.RemoveDisabledAttributes(res, d.resourceAttributes)
	return res, schemaURL, nil
}
