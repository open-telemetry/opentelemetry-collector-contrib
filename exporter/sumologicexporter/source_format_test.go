// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sumologicexporter

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
)

func getTestSourceFormat(template string) sourceFormat {
	r := regexp.MustCompile(sourceRegex)

	return newSourceFormat(r, template)
}

func TestNewSourceFormat(t *testing.T) {
	expected := sourceFormat{
		matches: []string{
			"test",
		},
		template: "%s/test",
	}

	r := regexp.MustCompile(sourceRegex)

	s := newSourceFormat(r, "%{test}/test")

	assert.Equal(t, expected, s)
}

func TestNewSourceFormats(t *testing.T) {
	expected := sourceFormats{
		host: sourceFormat{
			matches: []string{
				"namespace",
			},
			template: "ns/%s",
		},
		name: sourceFormat{
			matches: []string{
				"pod",
			},
			template: "name/%s",
		},
		category: sourceFormat{
			matches: []string{
				"cluster",
			},
			template: "category/%s",
		},
	}

	cfg := &Config{
		SourceName:     "name/%{pod}",
		SourceHost:     "ns/%{namespace}",
		SourceCategory: "category/%{cluster}",
	}

	s := newSourceFormats(cfg)

	assert.Equal(t, expected, s)
}

func TestFormat(t *testing.T) {
	f := fieldsFromMap(map[string]string{
		"key_1":        "value_1",
		"key_2.subkey": "value_2",
	})
	s := getTestSourceFormat("%{key_1}/%{key_2.subkey}")
	expected := "value_1/value_2"

	result := s.format(f)
	assert.Equal(t, expected, result)
}

func TestFormatNonExistingKey(t *testing.T) {
	f := fieldsFromMap(map[string]string{"key_2": "value_2"})
	s := getTestSourceFormat("%{key_1}/%{key_2}")

	expected := "/value_2"

	result := s.format(f)
	assert.Equal(t, expected, result)
}

func TestIsSet(t *testing.T) {
	s := getTestSourceFormat("%{key_1}/%{key_2}")
	assert.True(t, s.isSet())
}

func TestIsNotSet(t *testing.T) {
	s := getTestSourceFormat("")
	assert.False(t, s.isSet())
}
