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

package alibabacloudlogserviceexporter

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// The code for sanitize is mostly copied from:
//  https://github.com/open-telemetry/opentelemetry-collector/blob/2e84285efc665798d76773b9901727e8836e9d8f/exporter/prometheusexporter/sanitize_test.go

func TestSanitize(t *testing.T) {
	require.Equal(t, "", sanitize(""), "")
	require.Equal(t, "key_test", sanitize("_test"))
	require.Equal(t, "key_0test", sanitize("0test"))
	require.Equal(t, "test", sanitize("test"))
	require.Equal(t, "test__", sanitize("test_/"))
}
