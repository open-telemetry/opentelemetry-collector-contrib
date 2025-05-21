// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awss3receiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awss3receiver"

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.uber.org/multierr"
)

// S3DownloaderConfig contains aws s3 downloader related config to controls things
// like bucket, prefix, batching, connections, retries, etc.
type S3DownloaderConfig struct {
	Region              string `mapstructure:"region"`
	S3Bucket            string `mapstructure:"s3_bucket"`
	S3Prefix            string `mapstructure:"s3_prefix"`
	S3Partition         string `mapstructure:"s3_partition"`
	FilePrefix          string `mapstructure:"file_prefix"`
	Endpoint            string `mapstructure:"endpoint"`
	EndpointPartitionID string `mapstructure:"endpoint_partition_id"`
	S3ForcePathStyle    bool   `mapstructure:"s3_force_path_style"`
}

// SQSConfig holds SQS queue configuration for receiving object change notifications.
type SQSConfig struct {
	// QueueURL is the URL of the SQS queue to receive S3 notifications.
	QueueURL string `mapstructure:"queue_url"`
	// Region specifies the AWS region of the SQS queue.
	Region string `mapstructure:"region"`
	// Endpoint is the optional custom endpoint for SQS (useful for testing).
	Endpoint string `mapstructure:"endpoint"`
	// WaitTimeSeconds specifies the duration (in seconds) for long polling SQS messages.
	// Maximum is 20 seconds. Default is 20 seconds.
	WaitTimeSeconds int64 `mapstructure:"wait_time_seconds"`
	// MaxNumberOfMessages specifies the maximum number of messages to receive in a single poll.
	// Valid values: 1-10. Default is 10.
	MaxNumberOfMessages int64 `mapstructure:"max_number_of_messages"`
}

// Notifications groups optional notification sources.
type Notifications struct {
	OpAMP *component.ID `mapstructure:"opampextension"`
}

// Encoding defines the encoding configuration for the file receiver.
type Encoding struct {
	Extension component.ID `mapstructure:"extension"`
	Suffix    string       `mapstructure:"suffix"`
}

// Config defines the configuration for the file receiver.
type Config struct {
	S3Downloader  S3DownloaderConfig `mapstructure:"s3downloader"`
	StartTime     string             `mapstructure:"starttime"`
	EndTime       string             `mapstructure:"endtime"`
	Encodings     []Encoding         `mapstructure:"encodings"`
	Notifications Notifications      `mapstructure:"notifications"`
	// SQS configures receiving S3 object change notifications via an SQS queue.
	SQS *SQSConfig `mapstructure:"sqs"`
}

const (
	S3PartitionMinute = "minute"
	S3PartitionHour   = "hour"
)

func createDefaultConfig() component.Config {
	return &Config{
		S3Downloader: S3DownloaderConfig{
			Region:              "us-east-1",
			S3Partition:         S3PartitionMinute,
			EndpointPartitionID: "aws",
		},
	}
}

func (c Config) Validate() error {
	var errs error
	if c.S3Downloader.S3Bucket == "" {
		errs = multierr.Append(errs, errors.New("bucket is required"))
	}
	if c.S3Downloader.S3Partition != S3PartitionHour && c.S3Downloader.S3Partition != S3PartitionMinute {
		errs = multierr.Append(errs, errors.New("s3_partition must be either 'hour' or 'minute'"))
	}

	// Check for valid time-based configuration
	hasStartTime := c.StartTime != ""
	hasEndTime := c.EndTime != ""
	hasSQS := c.SQS != nil

	if !hasStartTime && !hasEndTime && !hasSQS {
		errs = multierr.Append(errs, errors.New("either starttime/endtime or sqs configuration must be provided"))
	}

	// If one of StartTime/EndTime is specified, the other must also be specified
	if hasStartTime && !hasEndTime {
		errs = multierr.Append(errs, errors.New("when starttime is specified, endtime is required"))
	}
	if !hasStartTime && hasEndTime {
		errs = multierr.Append(errs, errors.New("when endtime is specified, starttime is required"))
	}

	// StartTime and SQS cannot be specified together
	if hasStartTime && hasSQS {
		errs = multierr.Append(errs, errors.New("starttime/endtime and sqs configuration cannot be used together"))
	}

	// Validate StartTime format if specified
	if hasStartTime {
		if _, err := parseTime(c.StartTime, "starttime"); err != nil {
			errs = multierr.Append(errs, err)
		}
	}
	// Validate EndTime format if specified
	if hasEndTime {
		if _, err := parseTime(c.EndTime, "endtime"); err != nil {
			errs = multierr.Append(errs, err)
		}
	}

	// Validate SQS notifications if configured
	if c.SQS != nil {
		if c.SQS.QueueURL == "" {
			errs = multierr.Append(errs, errors.New("sqs.queue_url is required"))
		}
		if c.SQS.Region == "" {
			errs = multierr.Append(errs, errors.New("sqs.region is required"))
		}
		// Validate wait time seconds
		if c.SQS.WaitTimeSeconds < 0 || c.SQS.WaitTimeSeconds > 20 {
			errs = multierr.Append(errs, errors.New("sqs.wait_time_seconds must be between 0 and 20"))
		}
		// Validate max number of messages
		if c.SQS.MaxNumberOfMessages < 0 || c.SQS.MaxNumberOfMessages > 10 {
			errs = multierr.Append(errs, errors.New("sqs.max_number_of_messages must be between 1 and 10"))
		}
	}
	return errs
}

func parseTime(timeStr, configName string) (time.Time, error) {
	layouts := []string{time.RFC3339, "2006-01-02 15:04", time.DateOnly}

	for _, layout := range layouts {
		if t, err := time.Parse(layout, timeStr); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unable to parse %s (%s), accepted formats: %s", configName, timeStr, strings.Join(layouts, ", "))
}
