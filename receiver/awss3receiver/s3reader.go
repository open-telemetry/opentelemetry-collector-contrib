// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awss3receiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awss3receiver"

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/itchyny/timefmt-go"
	"go.uber.org/zap"
)

type s3TimeBasedReader struct {
	logger *zap.Logger

	listObjectsClient              ListObjectsAPI
	getObjectClient                GetObjectAPI
	s3Bucket                       string
	s3Prefix                       string
	s3PartitionFormat              string
	S3PartitionTimeLocation        *time.Location
	filePrefix                     string
	filePrefixIncludeTelemetryType bool
	startTime                      time.Time
	endTime                        time.Time
	notifier                       statusNotifier
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

	var s3PartitionTimeLocation *time.Location
	if cfg.S3Downloader.S3PartitionTimezone != "" {
		s3PartitionTimeLocation, err = time.LoadLocation(cfg.S3Downloader.S3PartitionTimezone)
		if err != nil {
			return nil, fmt.Errorf("invalid S3 partition timezone: %w", err)
		}
	} else {
		s3PartitionTimeLocation = time.Local
	}

	return &s3TimeBasedReader{
		logger:                         logger,
		listObjectsClient:              listObjectsClient,
		getObjectClient:                getObjectClient,
		s3Bucket:                       cfg.S3Downloader.S3Bucket,
		s3Prefix:                       cfg.S3Downloader.S3Prefix,
		filePrefix:                     cfg.S3Downloader.FilePrefix,
		filePrefixIncludeTelemetryType: cfg.S3Downloader.FilePrefixIncludeTelemetryType,
		s3PartitionFormat:              cfg.S3Downloader.S3PartitionFormat,
		S3PartitionTimeLocation:        s3PartitionTimeLocation,
		startTime:                      startTime,
		endTime:                        endTime,
		notifier:                       notifier,
	}, nil
}

// readAll implements the s3Reader interface
func (s3Reader *s3TimeBasedReader) readAll(ctx context.Context, telemetryType string, dataCallback s3ObjectCallback) error {
	timeStep, err := determineTimestep(s3Reader.s3PartitionFormat)
	if err != nil {
		return err
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
	timeKey := getTimeKey(s3Reader.s3PartitionFormat, t, s3Reader.S3PartitionTimeLocation)
	var telemetryTypeSuffix string
	if s3Reader.filePrefixIncludeTelemetryType {
		telemetryTypeSuffix = fmt.Sprintf("%s_", telemetryType)
	}
	// Retrieve the configured S3 prefix (may be empty, "/", "//", "logs/", etc.)
	prefix := s3Reader.s3Prefix

	// Case 1: No prefix provided → use only timeKey + filePrefix
	if prefix == "" {
		return fmt.Sprintf("%s/%s%s", timeKey, s3Reader.filePrefix, telemetryTypeSuffix)
	}
	// Case 2: Prefix contains only slashes (e.g., "/", "//", "///")
	// Keep the exact number of slashes and directly append timeKey without adding an extra "/"
	if strings.Trim(prefix, "/") == "" {
		return fmt.Sprintf("%s%s/%s%s", prefix, timeKey, s3Reader.filePrefix, telemetryTypeSuffix)
	}
	// Case 3: Normal prefix (e.g., "logs", "logs/", "/logs/", "//raw//")
	// Always add a "/" between prefix and timeKey to build a valid S3 path
	return fmt.Sprintf("%s/%s/%s%s", prefix, timeKey, s3Reader.filePrefix, telemetryTypeSuffix)
}

func (s3Reader *s3TimeBasedReader) sendStatus(ctx context.Context, status statusNotification) {
	if s3Reader.notifier != nil {
		s3Reader.notifier.SendStatus(ctx, status)
	}
}

func getTimeKey(partitionFormat string, t time.Time, location *time.Location) string {
	return timefmt.Format(t.In(location), partitionFormat)
}

func determineTimestep(partitionFormat string) (time.Duration, error) {
	startTime := time.Date(2025, time.December, 5, 11, 30, 0, 0, time.UTC)
	startTimeKey := getTimeKey(partitionFormat, startTime, time.UTC)
	for _, step := range []time.Duration{time.Minute, time.Hour} {
		nextTime := startTime.Add(step)
		nextTimeKey := getTimeKey(partitionFormat, nextTime, time.UTC)
		if startTimeKey != nextTimeKey {
			return step, nil
		}
	}
	return time.Duration(0), fmt.Errorf("no time step found for partition format %s", partitionFormat)
}
