// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"log"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	xrayv2 "github.com/aws/aws-xray-sdk-go/v2/instrumentation/awsv2"
	"github.com/aws/aws-xray-sdk-go/v2/xray"
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
	ctx, seg := initSegment(context.Background(), "DDB")

	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion("us-west-2"))
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}

	xrayv2.AWSV2Instrumentor(&cfg.APIOptions)

	dynamo = dynamodb.NewFromConfig(cfg)

	seg.User = "xraysegmentdump"
	seg.Close(ddbExpectedFailure(ctx))
}

func initSegment(ctx context.Context, name string) (context.Context, *xray.Segment) {
	respCtx, seg := xray.BeginSegment(ctx, name)
	defer seg.Close(nil)
	return respCtx, seg
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
