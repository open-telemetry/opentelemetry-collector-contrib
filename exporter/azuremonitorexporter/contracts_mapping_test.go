// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azuremonitorexporter

import (
	"testing"

	"github.com/microsoft/ApplicationInsights-Go/appinsights/contracts"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func TestResolveMapping(t *testing.T) {
	attrs := pcommon.NewMap()
	attrs.PutStr("host.name", "host-1")
	attrs.PutStr("service.instance.id", "instance-1")
	attrs.PutStr("empty.value", "")

	tests := []struct {
		name      string
		sources   []string
		wantValue string
		wantOK    bool
	}{
		{
			name:   "nil sources returns no value",
			wantOK: false,
		},
		{
			name:    "empty sources returns no value",
			sources: []string{},
			wantOK:  false,
		},
		{
			name:      "first source resolves",
			sources:   []string{"host.name", "service.instance.id"},
			wantValue: "host-1",
			wantOK:    true,
		},
		{
			name:      "falls through missing attribute to next source",
			sources:   []string{"not.present", "service.instance.id"},
			wantValue: "instance-1",
			wantOK:    true,
		},
		{
			name:      "skips empty-string attribute value",
			sources:   []string{"empty.value", "host.name"},
			wantValue: "host-1",
			wantOK:    true,
		},
		{
			name:      "all attributes missing, terminal literal selected",
			sources:   []string{"a.b", "c.d", "unknown-instance"},
			wantValue: "unknown-instance",
			wantOK:    true,
		},
		{
			name:      "literal at start short-circuits remaining sources",
			sources:   []string{"literal", "host.name"},
			wantValue: "literal",
			wantOK:    true,
		},
		{
			name:    "no literal, all attributes missing, no value",
			sources: []string{"a.b", "c.d"},
			wantOK:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := resolveMapping(attrs, tt.sources)
			assert.Equal(t, tt.wantOK, ok)
			assert.Equal(t, tt.wantValue, got)
		})
	}
}

func TestApplyCloudTagsToEnvelope_DefaultBehavior(t *testing.T) {
	// nil mappings ⇒ historical hardcoded behavior:
	// service.instance.id alone populates CloudRoleInstance.
	envelope := contracts.NewEnvelope()
	envelope.Tags = map[string]string{}
	attrs := pcommon.NewMap()
	attrs.PutStr("service.name", "svc")
	attrs.PutStr("service.instance.id", "instance-1")
	attrs.PutStr("host.name", "host-1")

	applyCloudTagsToEnvelope(envelope, attrs, nil)

	assert.Equal(t, "svc", envelope.Tags[contracts.CloudRole])
	assert.Equal(t, "instance-1", envelope.Tags[contracts.CloudRoleInstance])
}

func TestApplyCloudTagsToEnvelope_DefaultBehavior_OmitsWhenServiceInstanceIDMissing(t *testing.T) {
	envelope := contracts.NewEnvelope()
	envelope.Tags = map[string]string{}
	attrs := pcommon.NewMap()
	attrs.PutStr("service.name", "svc")
	attrs.PutStr("host.name", "host-1")

	applyCloudTagsToEnvelope(envelope, attrs, nil)

	_, present := envelope.Tags[contracts.CloudRoleInstance]
	assert.False(t, present, "default behavior must not fall back to host.name silently")
}

func TestApplyCloudTagsToEnvelope_MappingOverride(t *testing.T) {
	envelope := contracts.NewEnvelope()
	envelope.Tags = map[string]string{}
	attrs := pcommon.NewMap()
	attrs.PutStr("service.name", "svc")
	attrs.PutStr("service.instance.id", "instance-1")
	attrs.PutStr("host.name", "host-1")

	tagMappings := &TagMappingsConfig{
		CloudRoleInstance: []string{"host.name", "service.instance.id"},
	}
	applyCloudTagsToEnvelope(envelope, attrs, tagMappings)

	assert.Equal(t, "host-1", envelope.Tags[contracts.CloudRoleInstance],
		"configured precedence must override historical default")
}

func TestApplyCloudTagsToEnvelope_MappingFallsThroughToLiteral(t *testing.T) {
	envelope := contracts.NewEnvelope()
	envelope.Tags = map[string]string{}
	attrs := pcommon.NewMap()
	attrs.PutStr("service.name", "svc")

	tagMappings := &TagMappingsConfig{
		CloudRoleInstance: []string{"host.name", "service.instance.id", "unknown-instance"},
	}
	applyCloudTagsToEnvelope(envelope, attrs, tagMappings)

	assert.Equal(t, "unknown-instance", envelope.Tags[contracts.CloudRoleInstance])
}

func TestApplyCloudTagsToEnvelope_MappingNoMatchOmitsTag(t *testing.T) {
	envelope := contracts.NewEnvelope()
	envelope.Tags = map[string]string{}
	attrs := pcommon.NewMap()

	tagMappings := &TagMappingsConfig{
		CloudRoleInstance: []string{"host.name", "service.instance.id"},
	}
	applyCloudTagsToEnvelope(envelope, attrs, tagMappings)

	_, present := envelope.Tags[contracts.CloudRoleInstance]
	assert.False(t, present)
}

func TestApplyApplicationTagsToEnvelope_DefaultBehavior(t *testing.T) {
	envelope := contracts.NewEnvelope()
	envelope.Tags = map[string]string{}
	attrs := pcommon.NewMap()
	attrs.PutStr("service.version", "1.2.3")

	applyApplicationTagsToEnvelope(envelope, attrs, nil)

	assert.Equal(t, "1.2.3", envelope.Tags[contracts.ApplicationVersion])
}

func TestApplyApplicationTagsToEnvelope_MappingOverride(t *testing.T) {
	envelope := contracts.NewEnvelope()
	envelope.Tags = map[string]string{}
	attrs := pcommon.NewMap()
	attrs.PutStr("service.version", "1.2.3")
	attrs.PutStr("my.custom.version", "9.9.9")

	tagMappings := &TagMappingsConfig{
		ApplicationVersion: []string{"my.custom.version", "service.version"},
	}
	applyApplicationTagsToEnvelope(envelope, attrs, tagMappings)

	assert.Equal(t, "9.9.9", envelope.Tags[contracts.ApplicationVersion])
}

func TestApplyApplicationTagsToEnvelope_MappingFallsThroughToLiteral(t *testing.T) {
	envelope := contracts.NewEnvelope()
	envelope.Tags = map[string]string{}
	attrs := pcommon.NewMap()

	tagMappings := &TagMappingsConfig{
		ApplicationVersion: []string{"service.version", "unknown"},
	}
	applyApplicationTagsToEnvelope(envelope, attrs, tagMappings)

	assert.Equal(t, "unknown", envelope.Tags[contracts.ApplicationVersion])
}
