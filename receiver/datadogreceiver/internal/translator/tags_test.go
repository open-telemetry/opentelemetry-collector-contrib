// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package translator

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	semconv "go.opentelemetry.io/otel/semconv/v1.30.0"
)

func TestGetMetricAttributes(t *testing.T) {
	cases := []struct {
		name                  string
		tags                  []string
		host                  string
		expectedResourceAttrs pcommon.Map
		expectedScopeAttrs    pcommon.Map
		expectedDpAttrs       pcommon.Map
	}{
		{
			name:                  "empty",
			tags:                  []string{},
			host:                  "",
			expectedResourceAttrs: pcommon.NewMap(),
			expectedScopeAttrs:    pcommon.NewMap(),
			expectedDpAttrs:       pcommon.NewMap(),
		},
		{
			name: "host",
			tags: []string{},
			host: "host",
			expectedResourceAttrs: newMapFromKV(t, map[string]any{
				string(semconv.HostNameKey): "host",
			}),
			expectedScopeAttrs: pcommon.NewMap(),
			expectedDpAttrs:    pcommon.NewMap(),
		},
		{
			name: "provides both host and tags where some tag keys have to replaced by otel conventions",
			tags: []string{"env:prod", "service:my-service", "version:1.0"},
			host: "host",
			expectedResourceAttrs: newMapFromKV(t, map[string]any{
				string(semconv.HostNameKey):                  "host",
				string(semconv.DeploymentEnvironmentNameKey): "prod",
				string(semconv.ServiceNameKey):               "my-service",
				string(semconv.ServiceVersionKey):            "1.0",
			}),
			expectedScopeAttrs: pcommon.NewMap(),
			expectedDpAttrs:    pcommon.NewMap(),
		},
		{
			name: "provides host, tags and unnamed tags",
			tags: []string{"env:prod", "foo"},
			host: "host",
			expectedResourceAttrs: newMapFromKV(t, map[string]any{
				string(semconv.HostNameKey):                  "host",
				string(semconv.DeploymentEnvironmentNameKey): "prod",
			}),
			expectedScopeAttrs: pcommon.NewMap(),
			expectedDpAttrs: newMapFromKV(t, map[string]any{
				"unnamed_foo": "foo",
			}),
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			pool := newStringPool()
			attrs := tagsToAttributes(c.tags, c.host, pool)

			assert.Equal(t, c.expectedResourceAttrs.Len(), attrs.resource.Len())
			for k := range c.expectedResourceAttrs.All() {
				ev, _ := c.expectedResourceAttrs.Get(k)
				av, ok := attrs.resource.Get(k)
				assert.True(t, ok)
				assert.Equal(t, ev, av)
			}

			assert.Equal(t, c.expectedScopeAttrs.Len(), attrs.scope.Len())
			for k := range c.expectedScopeAttrs.All() {
				ev, _ := c.expectedScopeAttrs.Get(k)
				av, ok := attrs.scope.Get(k)
				assert.True(t, ok)
				assert.Equal(t, ev, av)
			}

			assert.Equal(t, c.expectedDpAttrs.Len(), attrs.dp.Len())
			for k := range c.expectedDpAttrs.All() {
				ev, _ := c.expectedDpAttrs.Get(k)
				av, ok := attrs.dp.Get(k)
				assert.True(t, ok)
				assert.Equal(t, ev, av)
			}
		})
	}
}

func newMapFromKV(t *testing.T, kv map[string]any) pcommon.Map {
	m := pcommon.NewMap()
	err := m.FromRaw(kv)
	assert.NoError(t, err)
	return m
}

func TestDatadogTagToKeyValuePair(t *testing.T) {
	cases := []struct {
		name          string
		input         string
		expectedKey   string
		expectedValue string
	}{
		{
			name:          "empty",
			input:         "",
			expectedKey:   "",
			expectedValue: "",
		},
		{
			name:          "kv tag",
			input:         "foo:bar",
			expectedKey:   "foo",
			expectedValue: "bar",
		},
		{
			name:          "unnamed tag",
			input:         "foo",
			expectedKey:   "unnamed_foo",
			expectedValue: "foo",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			key, value := translateDatadogTagToKeyValuePair(c.input)
			assert.Equal(t, c.expectedKey, key, "Expected key %s, got %s", c.expectedKey, key)
			assert.Equal(t, c.expectedValue, value, "Expected value %s, got %s", c.expectedValue, value)
		})
	}
}

func TestTranslateDataDogKeyToOtel(t *testing.T) {
	// make sure all known keys are translated
	for k, v := range datadogKnownResourceAttributes {
		t.Run(k, func(t *testing.T) {
			assert.Equal(t, v, translateDatadogKeyToOTel(k))
		})
	}

	// test dynamic attributes:
	// * http.request.header.<header_name>
	// * http.response.header.<header_name>
	assert.Equal(t, "http.request.header.referer", translateDatadogKeyToOTel("http.request.headers.referer"))
	assert.Equal(t, "http.response.header.content-type", translateDatadogKeyToOTel("http.response.headers.content-type"))
}

func TestImageTags(t *testing.T) {
	// make sure container.image.tags is a string[]
	expected := "[\"tag1\"]"
	tags := []string{"env:prod", "foo", "image_tag:tag1"}
	host := "host"
	pool := newStringPool()

	attrs := tagsToAttributes(tags, host, pool)
	imageTags, _ := attrs.resource.Get(string(semconv.ContainerImageTagsKey))
	assert.Equal(t, expected, imageTags.AsString())
}

func TestHTTPHeaders(t *testing.T) {
	// make sure container.image.tags is a string[]
	expected := "[\"value\"]"
	tags := []string{"env:prod", "foo", "http.request.headers.header:value", "http.response.headers.header:value"}
	host := "host"
	pool := newStringPool()

	attrs := tagsToAttributes(tags, host, pool)
	header, found := attrs.resource.Get("http.request.header.header")
	assert.True(t, found)
	assert.Equal(t, expected, header.AsString())
	header, found = attrs.resource.Get("http.response.header.header")
	assert.True(t, found)
	assert.Equal(t, expected, header.AsString())
}
