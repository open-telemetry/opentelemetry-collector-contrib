// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awss3receiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awss3receiver"

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.uber.org/zap"
)

type s3PollingReader struct {
	logger                         *zap.Logger
	listObjectsClient              ListObjectsAPI
	singleObjectClient             SingleObjectAPI
	s3Bucket                       string
	s3Prefix                       string
	s3PartitionFormat              string
	s3PartitionTimeLocation        *time.Location
	filePrefix                     string
	filePrefixIncludeTelemetryType bool
	startTime                      *time.Time
	pollInterval                   time.Duration
	lookback                       time.Duration
	timeStep                       time.Duration
	nowFunc                        func() time.Time
	tickerFunc                     func(d time.Duration) (<-chan time.Time, func())
	tagObjectAfterIngestion        bool
	skipIngestingTaggedObjects     bool
}

func newS3PollingReader(ctx context.Context, logger *zap.Logger, cfg *Config) (*s3PollingReader, error) {
	listObjectsClient, singleObjectClient, err := newS3Client(ctx, cfg.S3Downloader)
	if err != nil {
		return nil, err
	}

	var startTime *time.Time
	if cfg.StartTime != "" {
		st, pErr := parseTime(cfg.StartTime, "starttime")
		if pErr != nil {
			return nil, pErr
		}
		startTime = &st
	}

	timeStep, err := determineTimestep(cfg.S3Downloader.S3PartitionFormat)
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

	lookback := cfg.Poll.Lookback
	if lookback == 0 {
		lookback = cfg.Poll.Interval
	}

	return &s3PollingReader{
		logger:                         logger,
		listObjectsClient:              listObjectsClient,
		singleObjectClient:             singleObjectClient,
		s3Bucket:                       cfg.S3Downloader.S3Bucket,
		s3Prefix:                       cfg.S3Downloader.S3Prefix,
		s3PartitionFormat:              cfg.S3Downloader.S3PartitionFormat,
		s3PartitionTimeLocation:        s3PartitionTimeLocation,
		filePrefix:                     cfg.S3Downloader.FilePrefix,
		filePrefixIncludeTelemetryType: cfg.S3Downloader.FilePrefixIncludeTelemetryType,
		startTime:                      startTime,
		pollInterval:                   cfg.Poll.Interval,
		lookback:                       lookback,
		timeStep:                       timeStep,
		nowFunc:                        time.Now,
		tickerFunc: func(d time.Duration) (<-chan time.Time, func()) {
			ticker := time.NewTicker(d)
			return ticker.C, ticker.Stop
		},
		tagObjectAfterIngestion:    cfg.S3Downloader.TagObjectAfterIngestion,
		skipIngestingTaggedObjects: cfg.S3Downloader.SkipIngestingTaggedObjects,
	}, nil
}

func (r *s3PollingReader) readAll(ctx context.Context, telemetryType string, dataCallback s3ObjectCallback) error {
	r.logger.Info("Starting continuous polling for S3 telemetry",
		zap.Duration("poll_interval", r.pollInterval),
		zap.Duration("lookback", r.lookback),
		zap.String("partition_format", r.s3PartitionFormat),
	)

	var (
		backfillCompleted bool
		currentBackfill   time.Time
	)

	if r.startTime != nil {
		currentBackfill = *r.startTime
	}

	runBackfillPass := func() error {
		now := r.nowFunc()
		if !currentBackfill.Before(now) {
			backfillCompleted = true
			return nil
		}
		r.logger.Info("Executing historical backfill",
			zap.Time("start_time", currentBackfill),
			zap.Time("backfill_until", now),
		)
		err := r.scanRange(ctx, currentBackfill, now, telemetryType, dataCallback, func(completedPartition time.Time) {
			currentBackfill = advancePartitionTime(completedPartition, r.timeStep, r.s3PartitionTimeLocation)
		})
		if err != nil {
			if errors.Is(err, context.Canceled) || ctx.Err() != nil {
				return ctx.Err()
			}
			r.logger.Error("Error during historical backfill",
				zap.Error(err),
				zap.Time("resuming_from", currentBackfill),
			)
			return err
		}
		backfillCompleted = true
		r.logger.Info("Finished historical backfill",
			zap.Time("backfill_until", now),
		)
		return nil
	}

	runRollingPass := func() error {
		now := r.nowFunc()
		windowStart := now.Add(-r.lookback)
		if err := r.scanRange(ctx, windowStart, now, telemetryType, dataCallback, nil); err != nil {
			if errors.Is(err, context.Canceled) || ctx.Err() != nil {
				return ctx.Err()
			}
			r.logger.Error("Error during polling pass", zap.Error(err))
			return err
		}
		return nil
	}

	// Initial pass:
	if r.startTime != nil {
		_ = runBackfillPass()
	} else {
		_ = runRollingPass()
	}

	// Exit promptly if context was canceled during the initial pass
	if ctx.Err() != nil {
		return ctx.Err()
	}

	// Continuous polling loop
	tickerChan, tickerStop := r.tickerFunc(r.pollInterval)
	defer tickerStop()

	for {
		select {
		case <-ctx.Done():
			r.logger.Info("Context canceled, stopping continuous polling")
			return ctx.Err()
		case <-tickerChan:
			if r.startTime != nil && !backfillCompleted {
				_ = runBackfillPass()
			} else {
				_ = runRollingPass()
			}
		}
	}
}

func (r *s3PollingReader) scanRange(
	ctx context.Context,
	start, end time.Time,
	telemetryType string,
	dataCallback s3ObjectCallback,
	onPartitionSuccess func(t time.Time),
) error {
	startAligned := truncateToPartitionBoundary(start, r.timeStep, r.s3PartitionTimeLocation)
	endAligned := truncateToPartitionBoundary(end, r.timeStep, r.s3PartitionTimeLocation)
	if startAligned.After(endAligned) {
		return nil
	}

	for currentTime := startAligned; !currentTime.After(endAligned); currentTime = advancePartitionTime(currentTime, r.timeStep, r.s3PartitionTimeLocation) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			r.logger.Debug("Polling telemetry for partition", zap.Time("time", currentTime))
			if err := r.readTelemetryForTime(ctx, currentTime, telemetryType, dataCallback); err != nil {
				return err
			}
			if onPartitionSuccess != nil {
				onPartitionSuccess(currentTime)
			}
		}
	}
	return nil
}

func (r *s3PollingReader) readTelemetryForTime(ctx context.Context, t time.Time, telemetryType string, dataCallback s3ObjectCallback) error {
	return readTelemetryForPartition(
		ctx,
		r.logger,
		r.listObjectsClient,
		r.singleObjectClient,
		partitionConfig{
			bucket:                         r.s3Bucket,
			prefix:                         r.s3Prefix,
			partitionFormat:                r.s3PartitionFormat,
			partitionTimeLocation:          r.s3PartitionTimeLocation,
			filePrefix:                     r.filePrefix,
			filePrefixIncludeTelemetryType: r.filePrefixIncludeTelemetryType,
			skipIngestingTaggedObjects:     r.skipIngestingTaggedObjects,
			tagObjectAfterIngestion:        r.tagObjectAfterIngestion,
		},
		t,
		telemetryType,
		dataCallback,
	)
}

func truncateToPartitionBoundary(t time.Time, step time.Duration, loc *time.Location) time.Time {
	if loc == nil {
		loc = time.Local
	}
	inLoc := t.In(loc)
	y, m, d := inLoc.Date()
	hour, minute, _ := inLoc.Clock()
	switch {
	case step <= time.Minute:
		return time.Date(y, m, d, hour, minute, 0, 0, loc)
	case step <= time.Hour:
		return time.Date(y, m, d, hour, 0, 0, 0, loc)
	default:
		return time.Date(y, m, d, 0, 0, 0, 0, loc)
	}
}

func advancePartitionTime(t time.Time, step time.Duration, loc *time.Location) time.Time {
	if step >= 24*time.Hour {
		if loc == nil {
			loc = time.Local
		}
		return t.In(loc).AddDate(0, 0, 1)
	}
	return t.Add(step)
}
