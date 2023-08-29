// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package translator

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
)

func TestServiceFromResource(t *testing.T) {
	resource := constructDefaultResource()

	service := makeService(resource)

	assert.NotNil(t, service)
	w := testWriters.borrow()
	require.NoError(t, w.Encode(service))
	jsonStr := w.String()
	testWriters.release(w)
	assert.True(t, strings.Contains(jsonStr, "semver:1.1.4"))
}

func TestServiceFromResourceWithNoServiceVersion(t *testing.T) {
	resource := constructDefaultResource()
	resource.Attributes().Remove(conventions.AttributeServiceVersion)
	service := makeService(resource)

	assert.NotNil(t, service)
	w := testWriters.borrow()
	require.NoError(t, w.Encode(service))
	jsonStr := w.String()
	testWriters.release(w)
	assert.True(t, strings.Contains(jsonStr, "v1"))
}

func TestServiceFromNullResource(t *testing.T) {
	service := makeService(pcommon.NewResource())

	assert.Nil(t, service)
}
