// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-xray-sdk-go/v2/instrumentation/awsv2"
	"github.com/aws/aws-xray-sdk-go/v2/xray"
)

const (
	existingTableName    = "xray_sample_table"
	nonExistingTableName = "does_not_exist"
)

type Record struct {
	ID  string `json:"id"`
	URL string `json:"url"`
}

func main() {
	cfg, err := config.LoadDefaultConfig(context.Background(), config.WithRegion("us-west-2"))
	if err != nil {
		panic(err)
	}
	awsv2.AWSV2Instrumentor(&cfg.APIOptions)

	client := dynamodb.NewFromConfig(cfg)

	ctx, seg := xray.BeginSegment(context.Background(), "DDB")
	seg.User = "xraysegmentdump"
	err = ddbExpectedFailure(ctx, client)
	seg.Close(err)
}

func ddbExpectedFailure(ctx context.Context, client *dynamodb.Client) error {
	err := xray.Capture(ctx, "DDB.DescribeExistingTableAndPutToMissingTable", func(ctx1 context.Context) error {
		err := xray.AddAnnotation(ctx1, "DDB.DescribeExistingTableAndPutToMissingTable.Annotation", "anno")
		if err != nil {
			return err
		}
		err = xray.AddMetadata(ctx1, "DDB.DescribeExistingTableAndPutToMissingTable.AddMetadata", "meta")
		if err != nil {
			return err
		}

		_, err = client.DescribeTable(ctx1, &dynamodb.DescribeTableInput{
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

		_, err = client.PutItem(ctx1, &dynamodb.PutItemInput{
			TableName: aws.String(nonExistingTableName),
			Item:      item,
		})

		return err
	})
	return err
}
