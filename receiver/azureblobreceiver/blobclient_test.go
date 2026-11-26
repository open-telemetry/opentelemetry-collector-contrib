// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureblobreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azureblobreceiver"

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestReadBlobStreamsResponse(t *testing.T) {
	const contents = "blob contents"
	payload := strings.NewReader(contents)
	body := &trackedReadCloser{Reader: payload}
	service, err := azblob.NewClientWithNoCredential(storageAccountURL, &azblob.ClientOptions{
		ClientOptions: azcore.ClientOptions{Transport: blobTransport(func(req *http.Request) (*http.Response, error) {
			assert.Equal(t, "/logs/blob.gz", req.URL.Path)
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Length": {"13"}, "Etag": {`"version"`}},
				Body:       body,
				Request:    req,
			}, nil
		})},
	})
	require.NoError(t, err)
	client := &azureBlobClient{serviceClient: service}

	reader, err := client.readBlob(t.Context(), "logs", "blob.gz")
	require.NoError(t, err)
	assert.Equal(t, len(contents), payload.Len())
	assert.Zero(t, body.closes)
	data, err := io.ReadAll(reader)
	require.NoError(t, err)
	assert.Equal(t, contents, string(data))
	require.NoError(t, reader.Close())
	assert.Equal(t, 1, body.closes)
}

type blobTransport func(*http.Request) (*http.Response, error)

func (f blobTransport) Do(req *http.Request) (*http.Response, error) { return f(req) }

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
