// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awss3receiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awss3receiver"

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"go.uber.org/zap"
)

type s3TimeBasedReader struct {
	logger *zap.Logger

	listObjectsClient ListObjectsAPI
	getObjectClient   GetObjectAPI
	s3Bucket          string
	s3Prefix          string
	s3Partition       string
	filePrefix        string
	startTime         time.Time
	endTime           time.Time
	notifier          statusNotifier
}

func newS3TimeBasedReader(ctx context.Context, notifier statusNotifier, logger *zap.Logger, cfg *Config) (*s3TimeBasedReader, error) {
	listObjectsClient, getObjectClient, err := newS3Client(ctx, cfg.S3Downloader)
	if err != nil {
		return nil, err
	}
	startTime, err := parseTime(cfg.StartTime, "starttime")
	if err != nil {
		return nil, err
	}
	endTime, err := parseTime(cfg.EndTime, "endtime")
	if err != nil {
		return nil, err
	}
	if cfg.S3Downloader.S3Partition != S3PartitionHour && cfg.S3Downloader.S3Partition != S3PartitionMinute {
		return nil, errors.New("s3_partition must be either 'hour' or 'minute'")
	}

	return &s3TimeBasedReader{
		logger:            logger,
		listObjectsClient: listObjectsClient,
		getObjectClient:   getObjectClient,
		s3Bucket:          cfg.S3Downloader.S3Bucket,
		s3Prefix:          cfg.S3Downloader.S3Prefix,
		filePrefix:        cfg.S3Downloader.FilePrefix,
		s3Partition:       cfg.S3Downloader.S3Partition,
		startTime:         startTime,
		endTime:           endTime,
		notifier:          notifier,
	}, nil
}

// readAll implements the s3Reader interface
func (s3Reader *s3TimeBasedReader) readAll(ctx context.Context, telemetryType string, dataCallback s3ObjectCallback) error {
	var timeStep time.Duration
	if s3Reader.s3Partition == "hour" {
		timeStep = time.Hour
	} else {
		timeStep = time.Minute
	}
	s3Reader.logger.Info("Start reading telemetry", zap.Time("start_time", s3Reader.startTime), zap.Time("end_time", s3Reader.endTime))
	for currentTime := s3Reader.startTime; currentTime.Before(s3Reader.endTime); currentTime = currentTime.Add(timeStep) {
		s3Reader.sendStatus(ctx, statusNotification{
			TelemetryType: telemetryType,
			IngestStatus:  IngestStatusIngesting,
			StartTime:     s3Reader.startTime,
			EndTime:       s3Reader.endTime,
			IngestTime:    currentTime,
		})

		select {
		case <-ctx.Done():
			s3Reader.sendStatus(ctx, statusNotification{
				TelemetryType:  telemetryType,
				IngestStatus:   IngestStatusFailed,
				StartTime:      s3Reader.startTime,
				EndTime:        s3Reader.endTime,
				IngestTime:     currentTime,
				FailureMessage: ctx.Err().Error(),
			})
			s3Reader.logger.Error("Context cancelled, stopping reading telemetry", zap.Time("time", currentTime))
			return ctx.Err()
		default:
			s3Reader.logger.Info("Reading telemetry", zap.Time("time", currentTime))
			if err := s3Reader.readTelemetryForTime(ctx, currentTime, telemetryType, dataCallback); err != nil {
				s3Reader.sendStatus(ctx, statusNotification{
					TelemetryType:  telemetryType,
					IngestStatus:   IngestStatusFailed,
					StartTime:      s3Reader.startTime,
					EndTime:        s3Reader.endTime,
					IngestTime:     currentTime,
					FailureMessage: err.Error(),
				})
				s3Reader.logger.Error("Error reading telemetry", zap.Error(err), zap.Time("time", currentTime))
				return err
			}
		}
	}
	s3Reader.sendStatus(ctx, statusNotification{
		TelemetryType: telemetryType,
		IngestStatus:  IngestStatusCompleted,
		StartTime:     s3Reader.startTime,
		EndTime:       s3Reader.endTime,
		IngestTime:    s3Reader.endTime,
	})
	s3Reader.logger.Info("Finished reading telemetry", zap.Time("start_time", s3Reader.startTime), zap.Time("end_time", s3Reader.endTime))
	return nil
}

func (s3Reader *s3TimeBasedReader) readTelemetryForTime(ctx context.Context, t time.Time, telemetryType string, dataCallback s3ObjectCallback) error {
	params := &s3.ListObjectsV2Input{
		Bucket: &s3Reader.s3Bucket,
	}
	prefix := s3Reader.getObjectPrefixForTime(t, telemetryType)
	params.Prefix = &prefix
	s3Reader.logger.Debug("Finding telemetry with prefix", zap.String("prefix", prefix))
	p := s3Reader.listObjectsClient.NewListObjectsV2Paginator(params)

	firstPage := true
	for p.HasMorePages() {
		page, err := p.NextPage(ctx)
		if err != nil {
			return err
		}
		if firstPage && len(page.Contents) == 0 {
			s3Reader.logger.Info("No telemetry found for time", zap.String("prefix", prefix), zap.Time("time", t))
		} else {
			for _, obj := range page.Contents {
				data, err := retrieveS3Object(ctx, s3Reader.getObjectClient, s3Reader.s3Bucket, *obj.Key)
				if err != nil {
					return err
				}
				s3Reader.logger.Debug("Retrieved telemetry", zap.String("key", *obj.Key))
				if err := dataCallback(ctx, *obj.Key, data); err != nil {
					return err
				}
			}
		}
		firstPage = false
	}
	return nil
}

func (s3Reader *s3TimeBasedReader) getObjectPrefixForTime(t time.Time, telemetryType string) string {
	var timeKey string
	switch s3Reader.s3Partition {
	case S3PartitionMinute:
		timeKey = getTimeKeyPartitionMinute(t)
	case S3PartitionHour:
		timeKey = getTimeKeyPartitionHour(t)
	}
	if s3Reader.s3Prefix != "" {
		return fmt.Sprintf("%s/%s/%s%s_", s3Reader.s3Prefix, timeKey, s3Reader.filePrefix, telemetryType)
	}
	return fmt.Sprintf("%s/%s%s_", timeKey, s3Reader.filePrefix, telemetryType)
}

func (s3Reader *s3TimeBasedReader) sendStatus(ctx context.Context, status statusNotification) {
	if s3Reader.notifier != nil {
		s3Reader.notifier.SendStatus(ctx, status)
	}
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
