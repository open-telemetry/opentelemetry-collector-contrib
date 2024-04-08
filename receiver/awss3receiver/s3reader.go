// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awss3receiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awss3receiver"

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type s3Reader struct {
	listObjectsClient ListObjectsAPI
	getObjectClient   GetObjectAPI
	s3Bucket          string
	s3Prefix          string
	filePrefix        string
	startTime         time.Time
	endTime           time.Time
	timeStep          time.Duration
	getTimeKey        func(time.Time) string
}

type s3ReaderDataCallback func(context.Context, string, []byte) error

func newS3Reader(cfg *Config) (*s3Reader, error) {
	listObjectsClient, getObjectClient, err := newS3Client(cfg.S3Downloader)
	if err != nil {
		return nil, err
	}
	startTime, err := parseTime(cfg.StartTime)
	if err != nil {
		return nil, err
	}
	endTime, err := parseTime(cfg.EndTime)
	if err != nil {
		return nil, err
	}

	var getTimeKey func(time.Time) string
	var timeStep time.Duration
	if cfg.S3Downloader.S3Partition == "hour" {
		getTimeKey = getTimeKeyPartitionHour
		timeStep = time.Hour
	} else {
		getTimeKey = getTimeKeyPartitionMinute
		timeStep = time.Minute
	}
	return &s3Reader{
		listObjectsClient: listObjectsClient,
		getObjectClient:   getObjectClient,
		s3Bucket:          cfg.S3Downloader.S3Bucket,
		s3Prefix:          cfg.S3Downloader.S3Prefix,
		filePrefix:        cfg.S3Downloader.FilePrefix,
		startTime:         startTime,
		endTime:           endTime,
		timeStep:          timeStep,
		getTimeKey:        getTimeKey,
	}, nil
}

func (s3Reader *s3Reader) readAll(ctx context.Context, telemetryType string, dataCallback s3ReaderDataCallback) error {
	for currentTime := s3Reader.startTime; currentTime.Before(s3Reader.endTime); currentTime = currentTime.Add(s3Reader.timeStep) {
		select {
		case <-ctx.Done():
			return nil
		default:
			if err := s3Reader.readTelemetryForTime(ctx, currentTime, telemetryType, dataCallback); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s3Reader *s3Reader) readTelemetryForTime(ctx context.Context, t time.Time, telemetryType string, dataCallback s3ReaderDataCallback) error {
	params := &s3.ListObjectsV2Input{
		Bucket: &s3Reader.s3Bucket,
	}
	prefix := s3Reader.getObjectPrefixForTime(t, telemetryType)
	params.Prefix = &prefix

	p := s3Reader.listObjectsClient.NewListObjectsV2Paginator(params)

	for p.HasMorePages() {
		page, err := p.NextPage(context.TODO())
		if err != nil {
			return err
		}
		for _, obj := range page.Contents {
			data, err := s3Reader.retrieveObject(*obj.Key)
			if err != nil {
				return err
			}
			if err := dataCallback(ctx, *obj.Key, data); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s3Reader *s3Reader) getObjectPrefixForTime(t time.Time, telemetryType string) string {
	timeKey := s3Reader.getTimeKey(t)
	if s3Reader.s3Prefix != "" {
		return fmt.Sprintf("%s/%s/%s%s_", s3Reader.s3Prefix, timeKey, s3Reader.filePrefix, telemetryType)
	}
	return fmt.Sprintf("%s/%s%s_", timeKey, s3Reader.filePrefix, telemetryType)
}

func (s3Reader *s3Reader) retrieveObject(key string) ([]byte, error) {
	params := s3.GetObjectInput{
		Bucket: &s3Reader.s3Bucket,
		Key:    &key,
	}
	output, err := s3Reader.getObjectClient.GetObject(context.TODO(), &params)
	if err != nil {
		return nil, err
	}
	contents, err := io.ReadAll(output.Body)
	if err != nil {
		return nil, err
	}
	return contents, nil
}

func getTimeKeyPartitionHour(t time.Time) string {
	year, month, day := t.Date()
	hour := t.Hour()
	return fmt.Sprintf("year=%d/month=%02d/day=%02d/hour=%02d", year, month, day, hour)
}

func getTimeKeyPartitionMinute(t time.Time) string {
	year, month, day := t.Date()
	hour, minute, _ := t.Clock()
	return fmt.Sprintf("year=%d/month=%02d/day=%02d/hour=%02d/minute=%02d", year, month, day, hour, minute)
}
