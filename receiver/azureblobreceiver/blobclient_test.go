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

package azureblobreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azureblobreceiver"

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

const (
	goodConnectionString = "DefaultEndpointsProtocol=https;AccountName=accountName;AccountKey=+idLkHYcL0MUWIKYHm2j4Q==;EndpointSuffix=core.windows.net"
	badConnectionString  = "DefaultEndpointsProtocol=https;AccountName=accountName;AccountKey=accountkey;EndpointSuffix=core.windows.net"
)

func TestNewBlobClient(t *testing.T) {
	blobClient, err := newBlobClient(goodConnectionString, zaptest.NewLogger(t))

	require.NoError(t, err)
	require.NotNil(t, blobClient)
	assert.NotNil(t, blobClient.serviceClient)
}

func TestNewBlobClientError(t *testing.T) {
	blobClient, err := newBlobClient(badConnectionString, zaptest.NewLogger(t))

	assert.Error(t, err)
	assert.Nil(t, blobClient)
}
