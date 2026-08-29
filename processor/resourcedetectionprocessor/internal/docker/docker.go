// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package docker // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/docker"

import (
	"context"
	"errors"

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

const (
	// TypeStr is type of detector.
	TypeStr = "docker"
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
	return &Detector{
		detector:              newResourceDetector(),
		logger:                p.Logger,
		resourceAttributes:    cfg.(Config).ResourceAttributes,
		failOnMissingMetadata: failOnMissingMetadata,
	}, nil
}

// Detect detects system metadata and returns a resource with the available ones
func (d *Detector) Detect(ctx context.Context) (pcommon.Resource, string, error) {
	// Detection runs unfiltered so that an empty result answers "is this process running in a
	// Docker container?"; the configured attributes are applied afterwards. A partial result
	// still came from a reachable daemon, so the bridge keeps what it did return.
	// fail_on_missing_metadata covers an unusable daemon, not individual fields absent from
	// its response.
	res, schemaURL, err := sdkbridge.Detect(ctx, d.detector)
	if err != nil {
		d.logger.Debug("docker metadata unavailable", zap.Error(err))
		return pcommon.NewResource(), "", err
	}

	// The SDK detector returns an empty resource both when the daemon is unreachable and when
	// no container matches this process's hostname.
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
