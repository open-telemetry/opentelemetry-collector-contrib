// Copyright 2019, OpenTelemetry Authors
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
	"go.opentelemetry.io/collector/model/pdata"
	conventions "go.opentelemetry.io/collector/model/semconv/v1.5.0"
)

func TestServiceFromResource(t *testing.T) {
	resource := constructDefaultResource()

	service := makeService(resource)

	assert.NotNil(t, service)
	w := testWriters.borrow()
	if err := w.Encode(service); err != nil {
		assert.Fail(t, "invalid json")
	}
	jsonStr := w.String()
	testWriters.release(w)
	assert.True(t, strings.Contains(jsonStr, "semver:1.1.4"))
}

func TestServiceFromResourceWithNoServiceVersion(t *testing.T) {
	resource := constructDefaultResource()
	resource.Attributes().Delete(conventions.AttributeServiceVersion)
	service := makeService(resource)

	assert.NotNil(t, service)
	w := testWriters.borrow()
	if err := w.Encode(service); err != nil {
		assert.Fail(t, "invalid json")
	}
	jsonStr := w.String()
	testWriters.release(w)
	assert.True(t, strings.Contains(jsonStr, "v1"))
}

func TestServiceFromNullResource(t *testing.T) {
	service := makeService(pdata.NewResource())

	assert.Nil(t, service)
}
