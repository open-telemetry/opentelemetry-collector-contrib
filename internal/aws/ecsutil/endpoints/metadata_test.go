// Copyright The OpenTelemetry Authors
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

package endpoints

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_ecsPrefersLatestTME(t *testing.T) {
	t.Setenv(TaskMetadataEndpointV3EnvVar, "http://3")
	t.Setenv(TaskMetadataEndpointV4EnvVar, "http://4")

	tme, err := GetTMEFromEnv()
	require.NoError(t, err)
	require.NotNil(t, tme)
	assert.Equal(t, "4", tme.Host)
}
