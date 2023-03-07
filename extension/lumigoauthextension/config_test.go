// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package lumigoauthextension

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap/confmaptest"
)

func TestLoadConfigExporterNoToken(t *testing.T) {
	_, err := confmaptest.LoadConf(filepath.Join("testdata", "valid_config_exporter_no_token.yml"))
	require.NoError(t, err)

}

func TestLoadConfigExporterWithToken(t *testing.T) {
	_, err := confmaptest.LoadConf(filepath.Join("testdata", "valid_config_exporter_with_token.yml"))
	require.NoError(t, err)
}

func TestLoadConfigReceiver(t *testing.T) {
	_, err := confmaptest.LoadConf(filepath.Join("testdata", "valid_config_receiver.yml"))
	require.NoError(t, err)
}
