// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
