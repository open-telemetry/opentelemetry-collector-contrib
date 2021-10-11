// Copyright  The OpenTelemetry Authors
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

package metadataconfig

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudspannerreceiver/internal/metadataparser"
)

// Kind of sanity check test of metadata.yaml used for production usage
func TestParsingMetadataYaml(t *testing.T) {
	content, err := os.ReadFile("metadata.yaml")

	require.NoError(t, err)

	metadataMap, err := metadataparser.ParseMetadataConfig(content)

	require.NoError(t, err)
	require.NotNil(t, metadataMap)
}
