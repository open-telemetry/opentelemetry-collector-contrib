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

type Notifications struct {
	OpAMP *component.ID `mapstructure:"opampextension"`
}

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
	if c.StartTime == "" {
		errs = multierr.Append(errs, errors.New("starttime is required"))
	} else {
		if _, err := parseTime(c.StartTime, "starttime"); err != nil {
			errs = multierr.Append(errs, err)
		}
	}
	if c.EndTime == "" {
		errs = multierr.Append(errs, errors.New("endtime is required"))
	} else {
		if _, err := parseTime(c.EndTime, "endtime"); err != nil {
			errs = multierr.Append(errs, err)
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
