// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package openshift // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/openshift"

import (
	"context"
	"errors"
	"reflect"

	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/processor"
	openshiftdetector "go.opentelemetry.io/contrib/detectors/openshift"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/openshift/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/sdkbridge"
)

const (
	// TypeStr is type of detector.
	TypeStr = "openshift"
)

// Ensure detector implements internal.Detector.
var _ internal.Detector = (*detector)(nil)

// NewDetector returns a detector which can detect resource attributes on OpenShift 4.
// Detection is delegated to the upstream SDK detector so that the attributes reported
// here match the ones the collector's own telemetry reports.
func NewDetector(set processor.Settings, dcfg internal.DetectorConfig, failOnMissingMetadata bool) (internal.Detector, error) {
	cfg := dcfg.(Config)

	var opts []openshiftdetector.Option
	if cfg.Address != "" {
		opts = append(opts, openshiftdetector.WithAddress(cfg.Address))
	}
	if cfg.Token != "" {
		opts = append(opts, openshiftdetector.WithToken(cfg.Token))
	}
	// Without any TLS settings the SDK detector trusts the certificate authority
	// projected into the pod, so only build a TLS config when the user set one.
	if !reflect.DeepEqual(cfg.TLSs, configtls.ClientConfig{}) {
		if !cfg.TLSs.Insecure && cfg.TLSs.CAFile == "" && cfg.TLSs.CAPem == "" {
			cfg.TLSs.CAFile = defaultCAPath
		}
		tlsCfg, err := cfg.TLSs.LoadTLSConfig(context.Background())
		if err != nil {
			return nil, err
		}
		opts = append(opts, openshiftdetector.WithTLSConfig(tlsCfg))
	}

	return &detector{
		detector:              openshiftdetector.NewResourceDetector(opts...),
		logger:                set.Logger,
		resourceAttributes:    cfg.ResourceAttributes,
		failOnMissingMetadata: failOnMissingMetadata,
	}, nil
}

type detector struct {
	detector              sdkresource.Detector
	logger                *zap.Logger
	resourceAttributes    metadata.ResourceAttributesConfig
	failOnMissingMetadata bool
}

func (d *detector) Detect(ctx context.Context) (pcommon.Resource, string, error) {
	// Detection runs unfiltered so that an empty result answers "is this process on an
	// OpenShift cluster?"; the configured attributes are applied afterwards. A partial
	// result still came from a reachable API server, so the bridge keeps what it did return.
	res, schemaURL, err := sdkbridge.Detect(ctx, d.detector)
	if err != nil {
		d.logger.Error("OpenShift detector metadata retrieval failed", zap.Error(err))
		if d.failOnMissingMetadata {
			return pcommon.NewResource(), "", err
		}
		return pcommon.NewResource(), "", nil
	}

	// The SDK detector reports an empty resource and no error when not running in a
	// cluster, or when the API server does not serve the OpenShift config API.
	if res.Attributes().Len() == 0 {
		d.logger.Debug("OpenShift detector: not running on an OpenShift cluster")
		if d.failOnMissingMetadata {
			return pcommon.NewResource(), "", errors.New("openshift metadata unavailable")
		}
		return pcommon.NewResource(), "", nil
	}

	sdkbridge.RemoveDisabledAttributes(res, d.resourceAttributes)
	return res, schemaURL, nil
}
