// Copyright 2022 OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package awss3exporter

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"math/rand"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/xitongsys/parquet-go-source/s3"
	"github.com/xitongsys/parquet-go/writer"
)

const (
	jsonFormat    = "json"
	parquetFormat = "parquet"
)

type S3Writer struct {
}

// generate the s3 time key based on partition configuration
func getTimeKey(partition string) string {
	var timeKey string
	t := time.Now()
	year, month, day := t.Date()
	hour, minute, _ := t.Clock()

	rand.Int()

	if partition == "hour" {
		timeKey = fmt.Sprintf("year=%d/month=%02d/day=%02d/hour=%02d/", year, month, day, hour)
	} else {
		timeKey = fmt.Sprintf("year=%d/month=%02d/day=%02d/hour=%02d/minute=%02d/", year, month, day, hour, minute)
	}
	return timeKey
}

func getS3Key(bucket string, keyPrefix string, partition string, filePrefix string, fileformat string) string {
	timeKey := getTimeKey(partition)
	randomID := rand.Int()

	s3Key := bucket + "/" + keyPrefix + "/" + timeKey + "/" + filePrefix + "_" + strconv.Itoa(randomID) + "." + fileformat

	return s3Key
}

// read input schema file and parse it to json structure
func (s3writer *S3Writer) parseParquetInputSchema() (string, error) {
	content, err := ioutil.ReadFile("./schema/parquet_input_schema")
	if err != nil {
		return string(""), nil
	}

	return string(content), nil
}

// read output schema file
func (s3writer *S3Writer) parseParquetOutputSchema() (string, error) {
	content, err := ioutil.ReadFile("./schema/parquet_output_schema")
	if err != nil {
		return string(""), nil
	}
	return string(content), nil
}

func (s3writer *S3Writer) WriteJson(buf []byte, config *Config) error {
	key := getS3Key(config.S3Uploader.S3Bucket,
		config.S3Uploader.S3Prefix, config.S3Uploader.S3Partition,
		config.S3Uploader.FilePrefix, jsonFormat)

	// create a reader from data data in memory
	reader := bytes.NewReader(buf)

	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(config.S3Uploader.Region)},
	)

	if err != nil {
		return err
	}

	uploader := s3manager.NewUploader(sess)

	_, err = uploader.Upload(&s3manager.UploadInput{
		Bucket: aws.String(config.S3Uploader.S3Bucket),
		Key:    aws.String(key),
		Body:   reader,
	})
	if err != nil {
		return err
	}

	return nil
}

func (s3writer *S3Writer) WriteParquet(metrics []*ParquetMetric, ctx context.Context, config *Config) error {
	key := getS3Key(config.S3Uploader.S3Bucket, config.S3Uploader.S3Prefix,
		config.S3Uploader.S3Partition, config.S3Uploader.FilePrefix, parquetFormat)

	// create new S3 file writer
	fw, err := s3.NewS3FileWriter(ctx, config.S3Uploader.S3Bucket, key, "bucket-owner-full-control", nil, &aws.Config{
		Region: aws.String(config.S3Uploader.Region)})
	if err != nil {
		return err
	}
	// create new parquet file writer
	parquetOutputSchema, err := s3writer.parseParquetOutputSchema()
	if err != nil {
		return err
	}
	pw, err := writer.NewParquetWriter(fw, parquetOutputSchema, config.BatchCount)
	if err != nil {
		return err
	}
	for _, v := range metrics {
		err = pw.Write(v)
		if err != nil {
			return err
		}
	}

	err = pw.WriteStop()
	if err != nil {
		return err
	}

	err = fw.Close()

	if err != nil {
		return err
	}
	return nil
}
