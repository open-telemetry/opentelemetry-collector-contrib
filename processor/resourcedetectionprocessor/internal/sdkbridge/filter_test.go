// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sdkbridge

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

// resourceAttributeConfig mirrors the shape mdatagen generates.
type resourceAttributeConfig struct {
	Enabled bool `mapstructure:"enabled"`
}

type resourceAttributesConfig struct {
	CloudProvider resourceAttributeConfig `mapstructure:"cloud.provider"`
	ServiceName   resourceAttributeConfig `mapstructure:"service.name"`
	InstanceID    resourceAttributeConfig `mapstructure:"azure.app_service.instance.id"`
}

func newResource(attrs map[string]string) pcommon.Resource {
	res := pcommon.NewResource()
	for k, v := range attrs {
		res.Attributes().PutStr(k, v)
	}
	return res
}

func TestRemoveDisabledAttributes(t *testing.T) {
	res := newResource(map[string]string{
		"cloud.provider":                "azure",
		"service.name":                  "my-site",
		"azure.app_service.instance.id": "abc",
		"cloud.region":                  "westeurope",
	})

	RemoveDisabledAttributes(res, resourceAttributesConfig{
		CloudProvider: resourceAttributeConfig{Enabled: true},
		ServiceName:   resourceAttributeConfig{Enabled: false},
		InstanceID:    resourceAttributeConfig{Enabled: true},
	})

	assert.Equal(t, map[string]any{
		"cloud.provider":                "azure",
		"azure.app_service.instance.id": "abc",
	}, res.Attributes().AsRaw())
}

func TestRemoveDisabledAttributes_Pointer(t *testing.T) {
	res := newResource(map[string]string{"cloud.provider": "azure", "service.name": "my-site"})
	RemoveDisabledAttributes(res, &resourceAttributesConfig{CloudProvider: resourceAttributeConfig{Enabled: true}})
	assert.Equal(t, map[string]any{"cloud.provider": "azure"}, res.Attributes().AsRaw())
}

func TestRemoveDisabledAttributes_NotAStruct(t *testing.T) {
	for _, rac := range []any{nil, "not a struct", 42, (*resourceAttributesConfig)(nil)} {
		res := newResource(map[string]string{"cloud.provider": "azure"})
		RemoveDisabledAttributes(res, rac)
		assert.Equal(t, 0, res.Attributes().Len())
	}
}

func TestEnabledAttributes_SkipsNonAttributeFields(t *testing.T) {
	type mixed struct {
		CloudProvider resourceAttributeConfig `mapstructure:"cloud.provider"`
		Untagged      resourceAttributeConfig
		Ignored       resourceAttributeConfig `mapstructure:"-"`
		NotAStruct    string                  `mapstructure:"some.string"`
		NoEnabled     struct{ Other int }     `mapstructure:"no.enabled"`
	}

	assert.Equal(t, map[string]bool{"cloud.provider": true}, enabledAttributes(mixed{
		CloudProvider: resourceAttributeConfig{Enabled: true},
	}))
}

// Dynamic attributes such as ec2's "ec2.tag.<key>" are added after this call, as they
// already are after ResourceBuilder.Emit(), and must survive it.
func TestRemoveDisabledAttributes_DynamicAttributesAddedAfter(t *testing.T) {
	res := newResource(map[string]string{"cloud.provider": "aws", "service.name": "svc"})
	RemoveDisabledAttributes(res, resourceAttributesConfig{
		CloudProvider: resourceAttributeConfig{Enabled: true},
	})
	res.Attributes().PutStr("ec2.tag.team", "platform")

	assert.Equal(t, map[string]any{
		"cloud.provider": "aws",
		"ec2.tag.team":   "platform",
	}, res.Attributes().AsRaw())
}
