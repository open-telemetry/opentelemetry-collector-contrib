// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package nova // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/openstack/nova"

import (
	"context"
	"fmt"
	"regexp"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/processor"
	conventions "go.opentelemetry.io/otel/semconv/v1.39.0"
	"go.uber.org/zap"

	novaprovider "github.com/open-telemetry/opentelemetry-collector-contrib/internal/metadataproviders/openstack/nova"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/openstack/nova/internal/metadata"
)

const (
	// TypeStr is the detector type id.
	TypeStr = "nova"

	// LabelPrefix is the attribute prefix for Nova metadata keys (user/tenant-provided).
	LabelPrefix = "openstack.nova.meta."
)

var _ internal.Detector = (*Detector)(nil)

// Detector queries the OpenStack Nova metadata service and emits resource attributes.
type Detector struct {
	logger                *zap.Logger
	rb                    *metadata.ResourceBuilder
	metadataProvider      novaprovider.Provider
	labelRegexes          []*regexp.Regexp
	failOnMissingMetadata bool
}

// NewDetector creates a Nova detector.
func NewDetector(set processor.Settings, dcfg internal.DetectorConfig) (internal.Detector, error) {
	cfg := dcfg.(Config)

	rs, err := compileRegexes(cfg.Labels)
	if err != nil {
		return nil, err
	}

	return &Detector{
		logger:                set.Logger,
		rb:                    metadata.NewResourceBuilder(cfg.ResourceAttributes),
		metadataProvider:      novaprovider.NewProvider(),
		labelRegexes:          rs,
		failOnMissingMetadata: cfg.FailOnMissingMetadata,
	}, nil
}

func (d *Detector) Detect(ctx context.Context) (resource pcommon.Resource, schemaURL string, err error) {
	if _, err = d.metadataProvider.InstanceID(ctx); err != nil {
		d.logger.Debug("OpenStack Nova metadata unavailable", zap.Error(err))
		if d.failOnMissingMetadata {
			return pcommon.NewResource(), "", err
		}
		return pcommon.NewResource(), "", nil
	}

	meta, err := d.metadataProvider.Get(ctx)
	if err != nil {
		return pcommon.NewResource(), "", fmt.Errorf("failed getting Nova metadata: %w", err)
	}

	hostname, err := d.metadataProvider.Hostname(ctx)
	if err != nil {
		return pcommon.NewResource(), "", fmt.Errorf("failed getting hostname: %w", err)
	}

	// Optional: EC2‑compatible instance type (don’t fail if missing)
	if instanceType, err := d.metadataProvider.InstanceType(ctx); err == nil && instanceType != "" {
		d.rb.SetHostType(instanceType)
	} else if err != nil {
		d.logger.Debug("EC2-compatible instance type unavailable", zap.Error(err))
	}

	d.rb.SetCloudProvider("openstack")
	d.rb.SetCloudPlatform("openstack_nova")
	d.rb.SetCloudAccountID(meta.ProjectID)
	d.rb.SetCloudAvailabilityZone(meta.AvailabilityZone)
	d.rb.SetHostID(meta.UUID)
	d.rb.SetHostName(hostname)
	res := d.rb.Emit()

	// Optional: capture selected meta labels under openstack.meta.<key>
	if len(d.labelRegexes) > 0 && meta.Meta != nil {
		attrs := res.Attributes()
		for k, v := range meta.Meta {
			if regexArrayMatch(d.labelRegexes, k) {
				attrs.PutStr(LabelPrefix+k, v)
			}
		}
	}

	return res, conventions.SchemaURL, nil
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
