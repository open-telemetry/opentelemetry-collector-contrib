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

package main

import (
	"context"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-xray-sdk-go/xray"
)

var dynamo *dynamodb.DynamoDB

const (
	existingTableName    = "xray_sample_table"
	nonExistingTableName = "does_not_exist"
)

type Record struct {
	ID  string `json:"id"`
	URL string `json:"url"`
}

func main() {
	dynamo = dynamodb.New(session.Must(session.NewSession(
		&aws.Config{
			Region: aws.String("us-west-2")},
	)))
	xray.AWS(dynamo.Client)

	ctx, seg := xray.BeginSegment(context.Background(), "DDB")
	seg.User = "xraysegmentdump"
	err := ddbExpectedFailure(ctx)
	seg.Close(err)
}

func ddbExpectedFailure(ctx context.Context) error {
	err := xray.Capture(ctx, "DDB.DescribeExistingTableAndPutToMissingTable", func(ctx1 context.Context) error {
		xray.AddAnnotation(ctx1, "DDB.DescribeExistingTableAndPutToMissingTable.Annotation", "anno")
		xray.AddMetadata(ctx1, "DDB.DescribeExistingTableAndPutToMissingTable.AddMetadata", "meta")

		_, err := dynamo.DescribeTableWithContext(ctx1, &dynamodb.DescribeTableInput{
			TableName: aws.String(existingTableName),
		})
		if err != nil {
			return err
		}

		r := Record{
			ID:  "ABC123",
			URL: "https://example.com/first/link",
		}

		item, err := dynamodbattribute.MarshalMap(&r)
		if err != nil {
			return err
		}

		_, err = dynamo.PutItemWithContext(ctx1, &dynamodb.PutItemInput{
			TableName: aws.String(nonExistingTableName),
			Item:      item,
		})

		return err
	})
	return err
}
