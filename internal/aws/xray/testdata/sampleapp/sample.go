// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

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
		err := xray.AddAnnotation(ctx1, "DDB.DescribeExistingTableAndPutToMissingTable.Annotation", "anno")
		if err != nil {
			return err
		}
		err = xray.AddMetadata(ctx1, "DDB.DescribeExistingTableAndPutToMissingTable.AddMetadata", "meta")
		if err != nil {
			return err
		}

		_, err = dynamo.DescribeTableWithContext(ctx1, &dynamodb.DescribeTableInput{
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
