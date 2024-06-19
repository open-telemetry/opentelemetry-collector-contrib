// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awss3exporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awss3exporter"

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"math/rand"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"

	"github.com/aws/aws-sdk-go-v2/service/s3"

	"go.opentelemetry.io/collector/config/configcompression"
)

type s3Writer struct {
}

// generate the s3 time key based on partition configuration
func getTimeKey(time time.Time, partition string) string {
	var timeKey string
	year, month, day := time.Date()
	hour, minute, _ := time.Clock()

	if partition == "hour" {
		timeKey = fmt.Sprintf("year=%d/month=%02d/day=%02d/hour=%02d", year, month, day, hour)
	} else {
		timeKey = fmt.Sprintf("year=%d/month=%02d/day=%02d/hour=%02d/minute=%02d", year, month, day, hour, minute)
	}
	return timeKey
}

func randomInRange(low, hi int) int {
	return low + rand.Intn(hi-low)
}

func getS3Key(time time.Time, keyPrefix string, partition string, filePrefix string, metadata string, fileFormat string, compression configcompression.Type) string {
	timeKey := getTimeKey(time, partition)
	randomID := randomInRange(100000000, 999999999)
	suffix := ""
	if fileFormat != "" {
		suffix = "." + fileFormat
	}

	s3Key := keyPrefix + "/" + timeKey + "/" + filePrefix + metadata + "_" + strconv.Itoa(randomID) + suffix

	// add ".gz" extension to files if compression is enabled
	if compression == configcompression.TypeGzip {
		s3Key += ".gz"
	}

	return s3Key
}

func getConfig(s3Config *Config) (aws.Config, error) {
	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion(s3Config.S3Uploader.Region),
	)
	if err != nil {
		return aws.Config{}, err
	}

	return cfg, nil
}

func (s3writer *s3Writer) writeBuffer(_ context.Context, buf []byte, config *Config, metadata string, format string) error {
	now := time.Now()
	key := getS3Key(now,
		config.S3Uploader.S3Prefix, config.S3Uploader.S3Partition,
		config.S3Uploader.FilePrefix, metadata, format, config.S3Uploader.Compression)

	encoding := ""
	var reader *bytes.Reader
	if config.S3Uploader.Compression == configcompression.TypeGzip {
		// set s3 uploader content encoding to "gzip"
		encoding = "gzip"
		var gzipContents bytes.Buffer

		// create a gzip from data
		gzipWriter := gzip.NewWriter(&gzipContents)
		_, err := gzipWriter.Write(buf)
		if err != nil {
			return err
		}
		gzipWriter.Close()

		reader = bytes.NewReader(gzipContents.Bytes())
	} else {
		// create a reader from data in memory
		reader = bytes.NewReader(buf)
	}

	s3Config, err := getConfig(config)

	uploader := s3.NewFromConfig(s3Config)

	_, err = uploader.UploadPart(context.Background(), &s3.UploadPartInput{
		Bucket: aws.String(config.S3Uploader.S3Bucket),
		Key:    aws.String(key),
		Body:   reader,
	})

	if err != nil {
		return err
	}

	return nil
}
