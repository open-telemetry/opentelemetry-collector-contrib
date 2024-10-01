// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureblobreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azureblobreceiver"

import (
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

const (
	goodConnectionString = "DefaultEndpointsProtocol=https;AccountName=accountName;AccountKey=+idLkHYcL0MUWIKYHm2j4Q==;EndpointSuffix=core.windows.net"
	badConnectionString  = "DefaultEndpointsProtocol=https;AccountName=accountName;AccountKey=accountkey;EndpointSuffix=core.windows.net"

	storageAccountURL = "https://accountName.blob.core.windows.net"
)

func TestNewBlobClientFromConnectionString(t *testing.T) {
	blobClient, err := newBlobClientFromConnectionString(goodConnectionString, zaptest.NewLogger(t))

	require.NoError(t, err)
	require.NotNil(t, blobClient)
	assert.NotNil(t, blobClient.serviceClient)
}

func TestNewBlobClientFromConnectionStringError(t *testing.T) {
	blobClient, err := newBlobClientFromConnectionString(badConnectionString, zaptest.NewLogger(t))

	assert.Error(t, err)
	assert.Nil(t, blobClient)
}

func TestNewBlobClientFromCredentials(t *testing.T) {
	var cred azcore.TokenCredential = (*azidentity.ClientSecretCredential)(nil)
	blobClient, err := newBlobClientFromCredential(storageAccountURL, cred, zaptest.NewLogger(t))

	require.NoError(t, err)
	require.NotNil(t, blobClient)
	assert.NotNil(t, blobClient.serviceClient)
}
