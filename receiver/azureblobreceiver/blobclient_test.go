// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

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
