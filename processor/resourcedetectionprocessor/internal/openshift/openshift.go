// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package openshift // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/openshift"

import (
	"context"
	"crypto/tls"
	"errors"
	"reflect"
	"strings"

	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/processor"
	openshiftdetector "go.opentelemetry.io/contrib/detectors/openshift"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
	conventions "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.uber.org/zap"

	ocp "github.com/open-telemetry/opentelemetry-collector-contrib/internal/metadataproviders/openshift"
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

	var (
		opts   []openshiftdetector.Option
		tlsCfg *tls.Config
	)
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
		var err error
		tlsCfg, err = cfg.TLSs.LoadTLSConfig(context.Background())
		if err != nil {
			return nil, err
		}
		opts = append(opts, openshiftdetector.WithTLSConfig(tlsCfg))
	}

	return &detector{
		detector:              openshiftdetector.NewResourceDetector(opts...),
		cfg:                   cfg,
		tlsCfg:                tlsCfg,
		logger:                set.Logger,
		resourceAttributes:    cfg.ResourceAttributes,
		failOnMissingMetadata: failOnMissingMetadata,
	}, nil
}

type detector struct {
	detector              sdkresource.Detector
	cfg                   Config
	tlsCfg                *tls.Config
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

	if !metadata.ProcessorResourcedetectionOpenshiftRemoveCloudNameRegionFeatureGate.IsEnabled() {
		if _, ok := res.Attributes().Get(string(conventions.CloudRegionKey)); !ok {
			region, err := d.legacyCloudNameRegion(ctx)
			if err != nil {
				d.logger.Debug("OpenShift detector: legacy cloud.region retrieval failed", zap.Error(err))
			} else if region != "" {
				res.Attributes().PutStr(string(conventions.CloudRegionKey), region)
			}
		}
	}

	sdkbridge.RemoveDisabledAttributes(res, d.resourceAttributes)
	return res, schemaURL, nil
}

// legacyCloudNameRegion returns the lower cased Azure or OpenStack cloudName, which the
// detector reported as cloud.region before it was ported to the SDK detector.
// It is removed together with the removeCloudNameRegion feature gate.
func (d *detector) legacyCloudNameRegion(ctx context.Context) (string, error) {
	address, token, tlsCfg := d.cfg.Address, d.cfg.Token, d.tlsCfg
	var err error
	if address == "" {
		if address, err = readSVCAddressFromENV(); err != nil {
			return "", err
		}
	}
	if token == "" {
		if token, err = readK8STokenFromFile(); err != nil {
			return "", err
		}
	}
	if tlsCfg == nil && strings.HasPrefix(address, "https://") {
		caCfg := configtls.ClientConfig{Config: configtls.Config{CAFile: defaultCAPath}}
		if tlsCfg, err = caCfg.LoadTLSConfig(ctx); err != nil {
			return "", err
		}
	}

	infra, err := ocp.NewProvider(address, token, tlsCfg).Infrastructure(ctx)
	if err != nil {
		return "", err
	}
	platform := infra.Status.PlatformStatus
	switch strings.ToLower(platform.Type) {
	case "azure":
		return strings.ToLower(platform.Azure.CloudName), nil
	case "openstack":
		return strings.ToLower(platform.OpenStack.CloudName), nil
	}
	return "", nil
}
