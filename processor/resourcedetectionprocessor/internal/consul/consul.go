// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build (!remove_all_resourcedetection_detectors || enable_detector_consul) && !omit_detector_consul

package consul // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/consul"

import (
	"context"
	"errors"
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/processor"
	consuldetector "go.opentelemetry.io/contrib/detectors/consul"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/consul/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/sdkbridge"
)

// metaAttributePrefix namespaces consul node metadata keys. The SDK detector always emits it;
// the collector keeps it only when the
// processor.resourcedetection.consul.prefixMetaAttributes feature gate is enabled.
const metaAttributePrefix = "consul.meta."

// newResourceDetector is overridden in tests to substitute a fake SDK detector.
// Options are set only when configured so that the Consul client's CONSUL_* environment
// defaults still apply.
var newResourceDetector = func(cfg Config) sdkresource.Detector {
	var opts []consuldetector.Option
	if cfg.Address != "" {
		opts = append(opts, consuldetector.WithAddress(cfg.Address))
	}
	if cfg.Datacenter != "" {
		opts = append(opts, consuldetector.WithDatacenter(cfg.Datacenter))
	}
	if cfg.Namespace != "" {
		opts = append(opts, consuldetector.WithNamespace(cfg.Namespace))
	}
	if cfg.Token != "" {
		opts = append(opts, consuldetector.WithToken(string(cfg.Token)))
	}
	if cfg.TokenFile != "" {
		opts = append(opts, consuldetector.WithTokenFile(cfg.TokenFile))
	}
	if len(cfg.MetaLabels) > 0 {
		opts = append(opts, consuldetector.WithMetaKeyFilter(func(key string) bool {
			_, ok := cfg.MetaLabels[key]
			return ok
		}))
	}
	return consuldetector.NewResourceDetector(opts...)
}

var _ internal.Detector = (*Detector)(nil)

// Detector is a Consul agent metadata detector. Detection is delegated to the upstream SDK
// detector so that the attributes reported here match the ones the collector's own telemetry
// reports.
type Detector struct {
	detector              sdkresource.Detector
	logger                *zap.Logger
	resourceAttributes    metadata.ResourceAttributesConfig
	failOnMissingMetadata bool
}

// NewDetector creates a new Consul metadata detector.
func NewDetector(p processor.Settings, dcfg internal.DetectorConfig, failOnMissingMetadata bool) (internal.Detector, error) {
	cfg := dcfg.(Config)

	return &Detector{
		detector:              newResourceDetector(cfg),
		logger:                p.Logger,
		resourceAttributes:    cfg.ResourceAttributes,
		failOnMissingMetadata: failOnMissingMetadata,
	}, nil
}

// Detect queries the local Consul agent and returns a populated resource.
func (d *Detector) Detect(ctx context.Context) (pcommon.Resource, string, error) {
	res, schemaURL, err := sdkbridge.Detect(ctx, d.detector)
	if err != nil {
		d.logger.Debug("consul metadata unavailable", zap.Error(err))
		if d.failOnMissingMetadata {
			return pcommon.NewResource(), "", err
		}
		return pcommon.NewResource(), "", nil
	}

	// The SDK detector reports an empty resource and no error when no agent can be reached.
	if res.Attributes().Len() == 0 {
		d.logger.Debug("consul detector: metadata unavailable or no agent reachable")
		if d.failOnMissingMetadata {
			return pcommon.NewResource(), "", errors.New("consul metadata unavailable")
		}
		return pcommon.NewResource(), "", nil
	}

	// RemoveDisabledAttributes drops undeclared keys, so node metadata is set aside and
	// re-added after it.
	meta := map[string]string{}
	for k, v := range res.Attributes().All() {
		if key, ok := strings.CutPrefix(k, metaAttributePrefix); ok {
			meta[key] = v.Str()
		}
	}

	sdkbridge.RemoveDisabledAttributes(res, d.resourceAttributes)

	prefix := ""
	if metadata.ProcessorResourcedetectionConsulPrefixMetaAttributesFeatureGate.IsEnabled() {
		prefix = metaAttributePrefix
	}
	for key, value := range meta {
		res.Attributes().PutStr(prefix+key, value)
	}

	return res, schemaURL, nil
}
