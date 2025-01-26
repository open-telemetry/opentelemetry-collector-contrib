// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-xray-sdk-go/instrumentation/awsv2"
	"github.com/aws/aws-xray-sdk-go/xray"
)

var dynamo *dynamodb.Client

const (
	existingTableName    = "xray_sample_table"
	nonExistingTableName = "does_not_exist"
)

type Record struct {
	ID  string `json:"id"`
	URL string `json:"url"`
}

func main() {
	ctx, seg := xray.BeginSegment(context.Background(), "DDB")
	defer seg.Close(nil)

	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion("us-west-2"))
	if err != nil {
		panic(err)
	}

	awsv2.AWSV2Instrumentor(&cfg.APIOptions)

	dynamo = dynamodb.NewFromConfig(cfg)

	seg.User = "xraysegmentdump"
	err = ddbExpectedFailure(ctx)
	if err != nil {
		seg.AddError(err)
	}
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

		_, err = dynamo.DescribeTable(ctx1, &dynamodb.DescribeTableInput{
			TableName: aws.String(existingTableName),
		})
		if err != nil {
			return err
		}

		r := Record{
			ID:  "ABC123",
			URL: "https://example.com/first/link",
		}

		item, err := attributevalue.MarshalMap(&r)
		if err != nil {
			return err
		}

		_, err = dynamo.PutItem(ctx1, &dynamodb.PutItemInput{
			TableName: aws.String(nonExistingTableName),
			Item:      item,
		})

		return err
	})
	return err
}
