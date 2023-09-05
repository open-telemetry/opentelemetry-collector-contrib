// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awss3exporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awss3exporter"

import (
	"bytes"
	"context"
	"fmt"
	"gzip"
	"math/rand"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
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

func getS3Key(time time.Time, keyPrefix string, partition string, filePrefix string, metadata string, fileformat string) string {
	timeKey := getTimeKey(time, partition)
	randomID := randomInRange(100000000, 999999999)

	s3Key := keyPrefix + "/" + timeKey + "/" + filePrefix + metadata + "_" + strconv.Itoa(randomID) + "." + fileformat

	return s3Key
}

func getSessionConfig(config *Config) *aws.Config {
	sessionConfig := &aws.Config{
		Region: aws.String(config.S3Uploader.Region),
	}

	endpoint := config.S3Uploader.Endpoint
	if endpoint != "" {
		sessionConfig.Endpoint = aws.String(endpoint)
	}

	return sessionConfig
}

func (s3writer *s3Writer) writeBuffer(_ context.Context, buf []byte, config *Config, metadata string, format string) error {
	now := time.Now()
	key := getS3Key(now,
		config.S3Uploader.S3Prefix, config.S3Uploader.S3Partition,
		config.S3Uploader.FilePrefix, metadata, format)

	var reader *bytes.Reader
	var contentType string

	if config.S3Uploader.Compression == "gzip" {
		// Create a buffer to hold compressed data
		var compressedBuffer bytes.Buffer
		writer := gzip.NewWriter(&compressedBuffer)

		// Write the original data to the gzip writer
		_, err := writer.Write(buf)
		if err != nil {
			return err
		}

		// Close the gzip writer to flush any remaining data to compressedBuffer
		err = writer.Close()
		if err != nil {
			return err
		}

		// Create a reader from compressed data in memory
		reader = bytes.NewReader(compressedBuffer.Bytes())
		contentType = "application/gzip"
		key += ".gz" // Added ".gz" to indicate it's gzipped
	} else {
		// Create a reader from data in memory (not compressed)
		reader = bytes.NewReader(buf)
		contentType = "application/octet-stream"
	}

	sessionConfig := getSessionConfig(config)
	sess, err := session.NewSession(sessionConfig)

	if err != nil {
		return err
	}

	uploader := s3manager.NewUploader(sess)

	_, err = uploader.Upload(&s3manager.UploadInput{
		Bucket:      aws.String(config.S3Uploader.S3Bucket),
		Key:         aws.String(key),
		Body:        reader,
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return err
	}

	return nil
}
