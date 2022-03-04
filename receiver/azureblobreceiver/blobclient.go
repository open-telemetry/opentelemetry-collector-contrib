// Copyright OpenTelemetry Authors
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

package azureblobreceiver

import (
	"bytes"
	"context"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"go.uber.org/zap"
)

type BlobClient interface {
	ReadBlob(ctx context.Context, containerName string, blobName string) (*bytes.Buffer, error)
	DeleteBlob(ctx context.Context, containerName string, blobName string) error
}

type AzureBlobClient struct {
	serviceClient azblob.ServiceClient
	logger        *zap.Logger
}

func (bc *AzureBlobClient) getBlockBlob(ctx context.Context, containerName string, blobName string) azblob.BlockBlobClient {
	containerClient := bc.serviceClient.NewContainerClient(containerName)
	return containerClient.NewBlockBlobClient(blobName)
}

func (bc *AzureBlobClient) ReadBlob(ctx context.Context, containerName string, blobName string) (*bytes.Buffer, error) {
	blockBlob := bc.getBlockBlob(ctx, containerName, blobName)

	get, err := blockBlob.Download(ctx, nil)
	if err != nil {
		return nil, err
	}

	downloadedData := &bytes.Buffer{}
	reader := get.Body(nil)
	defer reader.Close()

	_, err = downloadedData.ReadFrom(reader)

	return downloadedData, err
}

func (bc *AzureBlobClient) DeleteBlob(ctx context.Context, containerName string, blobName string) error {
	return nil
}

// const (
// 	containerNotFoundError = "ErrorCode=ContainerNotFound"
// )

// func (bc *AzureBlobClient) generateBlobName(dataType config.DataType) string {
// 	return fmt.Sprintf("%s-%s", dataType, uuid.NewString())
// }

// func (bc *AzureBlobClient) checkOrCreateContainer() error {
// 	_, err := bc.containerClient.GetProperties(context.TODO(), nil)
// 	if err != nil && strings.Contains(err.Error(), containerNotFoundError) {
// 		_, err = bc.containerClient.Create(context.TODO(), nil)
// 	}
// 	return err
// }

// func (bc *AzureBlobClient) UploadData(data []byte, dataType config.DataType) error {
// 	blobName := bc.generateBlobName(dataType)

// 	blockBlob := bc.containerClient.NewBlockBlobClient(blobName)

// 	err := bc.checkOrCreateContainer()
// 	if err != nil {
// 		return err
// 	}

// 	_, err = blockBlob.Upload(context.TODO(), streaming.NopCloser(bytes.NewReader(data)), nil)

// 	return err
// }

func NewBlobClient(connectionString string, logger *zap.Logger) (*AzureBlobClient, error) {
	serviceClient, err := azblob.NewServiceClientFromConnectionString(connectionString, nil)
	if err != nil {
		return nil, err
	}

	return &AzureBlobClient{
		serviceClient,
		logger,
	}, nil
}
