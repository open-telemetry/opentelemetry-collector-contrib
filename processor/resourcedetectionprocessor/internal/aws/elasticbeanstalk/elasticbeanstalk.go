// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package elasticbeanstalk // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/aws/elasticbeanstalk"

import (
	"context"
	"errors"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/processor"
	ebdetector "go.opentelemetry.io/contrib/detectors/aws/elasticbeanstalk"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
	conventions "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/aws/elasticbeanstalk/internal/metadata"
)

const (
	// TypeStr is type of detector.
	TypeStr = "elastic_beanstalk"
)

// newResourceDetector is overridden in tests to substitute a fake SDK detector.
var newResourceDetector = func() sdkresource.Detector {
	return ebdetector.NewResourceDetector()
}

var _ internal.Detector = (*Detector)(nil)

// Detector is an Elastic Beanstalk detector. Detection is delegated to the
// upstream SDK detector so that the attributes reported here match the ones the
// collector's own telemetry reports.
type Detector struct {
	detector              sdkresource.Detector
	logger                *zap.Logger
	rb                    *metadata.ResourceBuilder
	failOnMissingMetadata bool
}

func NewDetector(p processor.Settings, dcfg internal.DetectorConfig, failOnMissingMetadata bool) (internal.Detector, error) {
	if metadata.ProcessorResourcedetectionElasticbeanstalkDontEmitV0DeploymentConventionsFeatureGate.IsEnabled() &&
		!metadata.ProcessorResourcedetectionElasticbeanstalkEmitV1DeploymentConventionsFeatureGate.IsEnabled() {
		return nil, errors.New("processor.resourcedetection.elasticbeanstalk.DontEmitV0DeploymentConventions requires processor.resourcedetection.elasticbeanstalk.EmitV1DeploymentConventions to be enabled")
	}

	cfg := dcfg.(Config)

	return &Detector{
		detector:              newResourceDetector(),
		logger:                p.Logger,
		rb:                    metadata.NewResourceBuilder(cfg.ResourceAttributes),
		failOnMissingMetadata: failOnMissingMetadata,
	}, nil
}

func (d *Detector) Detect(ctx context.Context) (pcommon.Resource, string, error) {
	res, err := d.detector.Detect(ctx)
	if err != nil && !errors.Is(err, sdkresource.ErrPartialResource) {
		// The configuration file is present but unreadable or malformed. This is
		// reported regardless of fail_on_missing_metadata, as it was before
		// detection moved to the SDK detector.
		return pcommon.NewResource(), "", err
	}

	// The SDK detector reports an empty resource and no error when the
	// configuration file is absent, which is the case both when X-Ray is
	// disabled and when not running on Elastic Beanstalk at all.
	if res.Len() == 0 {
		d.logger.Debug("Elastic Beanstalk detector: configuration file unavailable or not running on Elastic Beanstalk")
		if d.failOnMissingMetadata {
			return pcommon.NewResource(), "", errors.New("elastic_beanstalk metadata unavailable")
		}
		return pcommon.NewResource(), "", nil
	}

	emitV1 := metadata.ProcessorResourcedetectionElasticbeanstalkEmitV1DeploymentConventionsFeatureGate.IsEnabled()
	dontEmitV0 := metadata.ProcessorResourcedetectionElasticbeanstalkDontEmitV0DeploymentConventionsFeatureGate.IsEnabled()

	for _, attr := range res.Attributes() {
		val := attr.Value.AsString()
		switch attr.Key {
		case conventions.CloudProviderKey:
			d.rb.SetCloudProvider(val)
		case conventions.CloudPlatformKey:
			d.rb.SetCloudPlatform(val)
		case conventions.ServiceVersionKey:
			d.rb.SetServiceVersion(val)
		case conventions.DeploymentEnvironmentNameKey:
			if emitV1 {
				d.rb.SetDeploymentEnvironmentName(val)
			}
			if !dontEmitV0 {
				d.rb.SetDeploymentEnvironment(val)
			}
		case conventions.DeploymentIDKey:
			if emitV1 {
				d.rb.SetDeploymentID(val)
			}
			if !dontEmitV0 {
				d.rb.SetServiceInstanceID(val)
			}
		}
	}

	return d.rb.Emit(), conventions.SchemaURL, nil
}
