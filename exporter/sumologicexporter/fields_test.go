// Copyright 2020 OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package sumologicexporter

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFieldsAsString(t *testing.T) {
	expected := "key1=value1, key2=value2, key3=value3"
	flds := fields{
		"key1": "value1",
		"key3": "value3",
		"key2": "value2",
	}

	assert.Equal(t, expected, flds.string())
}
