// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build (!remove_all_resourcedetection_detectors || enable_detector_openstack_nova) && !omit_detector_openstack_nova

package nova // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/openstack/nova"

import (
	"context"
	"errors"
	"regexp"
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/processor"
	novadetector "go.opentelemetry.io/contrib/detectors/openstack/nova"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/openstack/nova/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/sdkbridge"
)

// LabelPrefix is the attribute prefix for Nova metadata keys (user/tenant-provided).
const LabelPrefix = "openstack.nova.meta."

// newResourceDetector is overridden in tests to substitute a fake SDK detector.
// metaKeyFilter is nil when no labels are configured.
var newResourceDetector = func(metaKeyFilter func(string) bool) sdkresource.Detector {
	if metaKeyFilter == nil {
		return novadetector.NewResourceDetector()
	}
	return novadetector.NewResourceDetector(novadetector.WithMetaKeyFilter(metaKeyFilter))
}

var _ internal.Detector = (*Detector)(nil)

// Detector is an OpenStack Nova metadata detector. Detection is delegated to the
// upstream SDK detector so that the attributes reported here match the ones the
// collector's own telemetry reports.
type Detector struct {
	detector              sdkresource.Detector
	logger                *zap.Logger
	resourceAttributes    metadata.ResourceAttributesConfig
	failOnMissingMetadata bool
}

// NewDetector creates a Nova detector.
func NewDetector(set processor.Settings, dcfg internal.DetectorConfig, failOnMissingMetadata bool) (internal.Detector, error) {
	cfg := dcfg.(Config)

	rs, err := compileRegexes(cfg.Labels)
	if err != nil {
		return nil, err
	}

	var metaKeyFilter func(string) bool
	if len(rs) > 0 {
		metaKeyFilter = func(key string) bool { return regexArrayMatch(rs, key) }
	}

	return &Detector{
		detector:              newResourceDetector(metaKeyFilter),
		logger:                set.Logger,
		resourceAttributes:    cfg.ResourceAttributes,
		failOnMissingMetadata: failOnMissingMetadata || cfg.FailOnMissingMetadata,
	}, nil
}

// Detect queries the Nova metadata service and returns a populated resource.
func (d *Detector) Detect(ctx context.Context) (pcommon.Resource, string, error) {
	// A partial result still came from a reachable metadata service, so the bridge keeps
	// what it did return. fail_on_missing_metadata covers an unusable metadata service,
	// not individual fields absent from its response.
	res, schemaURL, err := sdkbridge.Detect(ctx, d.detector)
	if err != nil {
		d.logger.Debug("OpenStack Nova metadata unavailable", zap.Error(err))
		if d.failOnMissingMetadata {
			return pcommon.NewResource(), "", err
		}
		return pcommon.NewResource(), "", nil
	}

	// The SDK detector reports an empty resource and no error both when the
	// metadata service is unreachable and when running outside OpenStack.
	if res.Attributes().Len() == 0 {
		d.logger.Debug("OpenStack Nova detector: metadata unavailable or not running on a Nova instance")
		if d.failOnMissingMetadata {
			return pcommon.NewResource(), "", errors.New("nova metadata unavailable")
		}
		return pcommon.NewResource(), "", nil
	}

	// RemoveDisabledAttributes drops undeclared keys, so the dynamic meta keys are put back after it.
	meta := pcommon.NewMap()
	for k, v := range res.Attributes().All() {
		if strings.HasPrefix(k, LabelPrefix) {
			v.CopyTo(meta.PutEmpty(k))
		}
	}
	sdkbridge.RemoveDisabledAttributes(res, d.resourceAttributes)
	for k, v := range meta.All() {
		v.CopyTo(res.Attributes().PutEmpty(k))
	}

	return res, schemaURL, nil
}

func compileRegexes(pats []string) ([]*regexp.Regexp, error) {
	if len(pats) == 0 {
		return nil, nil
	}
	out := make([]*regexp.Regexp, 0, len(pats))
	for _, p := range pats {
		re, err := regexp.Compile(p)
		if err != nil {
			return nil, err
		}
		out = append(out, re)
	}
	return out, nil
}

func regexArrayMatch(arr []*regexp.Regexp, s string) bool {
	for _, r := range arr {
		if r.MatchString(s) {
			return true
		}
	}
	return false
}
