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
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"go.uber.org/zap"
)

// type BlobClient interface {
// 	UploadData(data []byte, dataType config.DataType) error
// }

type AzureBlobClient struct {
	containerClient azblob.ContainerClient
	logger          *zap.Logger
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

// func NewBlobClient(connectionString string, containerName string, logger *zap.Logger) (*AzureBlobClient, error) {
// 	serviceClient, err := azblob.NewServiceClientFromConnectionString(connectionString, nil)
// 	if err != nil {
// 		return nil, err
// 	}

// 	containerClient := serviceClient.NewContainerClient(containerName)

// 	return &AzureBlobClient{
// 		containerClient,
// 		logger,
// 	}, nil
// }
