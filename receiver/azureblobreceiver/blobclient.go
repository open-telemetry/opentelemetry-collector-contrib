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
	"bytes"
	"context"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"go.uber.org/zap"
)

type blobClient interface {
	readBlob(ctx context.Context, containerName string, blobName string) (*bytes.Buffer, error)
}

type azureBlobClient struct {
	serviceClient *azblob.Client
	logger        *zap.Logger
}

var _ blobClient = (*azureBlobClient)(nil)

func (bc *azureBlobClient) readBlob(ctx context.Context, containerName string, blobName string) (*bytes.Buffer, error) {

	defer func() {
		_, blobDeleteErr := bc.serviceClient.DeleteBlob(ctx, containerName, blobName, nil)
		if blobDeleteErr != nil {
			bc.logger.Error("failed to delete blob", zap.Error(blobDeleteErr))
		}
	}()

	get, err := bc.serviceClient.DownloadStream(ctx, containerName, blobName, nil)
	if err != nil {
		return nil, err
	}

	downloadedData := &bytes.Buffer{}
	retryReader := get.NewRetryReader(ctx, &azblob.RetryReaderOptions{})
	defer retryReader.Close()

	_, err = downloadedData.ReadFrom(retryReader)

	return downloadedData, err
}

func newBlobClient(connectionString string, logger *zap.Logger) (*azureBlobClient, error) {
	serviceClient, err := azblob.NewClientFromConnectionString(connectionString, nil)
	if err != nil {
		return nil, err
	}

	return &azureBlobClient{
		serviceClient,
		logger,
	}, nil
}
