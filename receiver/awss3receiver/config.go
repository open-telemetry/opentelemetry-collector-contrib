// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awss3receiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awss3receiver"

import (
	"errors"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.uber.org/multierr"
)

// S3DownloaderConfig contains aws s3 downloader related config to controls things
// like bucket, prefix, batching, connections, retries, etc.
type S3DownloaderConfig struct {
	Region           string `mapstructure:"region"`
	S3Bucket         string `mapstructure:"s3_bucket"`
	S3Prefix         string `mapstructure:"s3_prefix"`
	S3Partition      string `mapstructure:"s3_partition"`
	FilePrefix       string `mapstructure:"file_prefix"`
	Endpoint         string `mapstructure:"endpoint"`
	S3ForcePathStyle bool   `mapstructure:"s3_force_path_style"`
}

// Config defines the configuration for the file receiver.
type Config struct {
	S3Downloader S3DownloaderConfig `mapstructure:"s3downloader"`
	StartTime    string             `mapstructure:"starttime"`
	EndTime      string             `mapstructure:"endtime"`
}

func createDefaultConfig() component.Config {
	return &Config{
		S3Downloader: S3DownloaderConfig{
			Region:      "us-east-1",
			S3Partition: "minute",
		},
	}
}

func (c Config) Validate() error {
	var errs error
	if c.S3Downloader.S3Bucket == "" {
		errs = multierr.Append(errs, errors.New("bucket is required"))
	}
	if c.StartTime == "" {
		errs = multierr.Append(errs, errors.New("start time is required"))
	} else {
		if err := validateTime(c.StartTime); err != nil {
			errs = multierr.Append(errs, errors.New("unable to parse start date"))
		}
	}
	if c.EndTime == "" {
		errs = multierr.Append(errs, errors.New("end time is required"))
	} else {
		if err := validateTime(c.EndTime); err != nil {
			errs = multierr.Append(errs, errors.New("unable to parse end time"))
		}
	}
	return errs
}

func validateTime(str string) error {
	layouts := []string{"2006-01-02 15:04", time.DateOnly}
	for _, layout := range layouts {
		if _, err := time.Parse(layout, str); err == nil {
			return nil
		}
	}
	return errors.New("unable to parse time string")
}
