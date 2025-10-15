// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awss3exporter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/config/configcompression"
)

func TestNewUploadManager(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name   string
		conf   *Config
		errVal string
	}{
		{
			name: "valid configuration",
			conf: &Config{
				S3Uploader: S3UploaderConfig{
					Region:              "local",
					S3Bucket:            "my-awesome-bucket",
					S3Prefix:            "opentelemetry",
					S3PartitionFormat:   "year=%Y/month=%m/day=%d/hour=%H",
					S3PartitionTimezone: "Europe/London",
					FilePrefix:          "ingested-data-",
					Endpoint:            "localhost",
					RoleArn:             "arn:aws:iam::123456789012:my-awesome-user",
					S3ForcePathStyle:    true,
					DisableSSL:          true,
					Compression:         configcompression.TypeGzip,
				},
			},
			errVal: "",
		},
		{
			name: "invalid timezone configuration",
			conf: &Config{
				S3Uploader: S3UploaderConfig{
					Region:              "local",
					S3Bucket:            "my-awesome-bucket",
					S3Prefix:            "opentelemetry",
					S3PartitionFormat:   "year=%Y/month=%m/day=%d/hour=%H",
					S3PartitionTimezone: "non-existing timezone",
					FilePrefix:          "ingested-data-",
					Endpoint:            "localhost",
					RoleArn:             "arn:aws:iam::123456789012:my-awesome-user",
					S3ForcePathStyle:    true,
					DisableSSL:          true,
					Compression:         configcompression.TypeGzip,
				},
			},
			errVal: "invalid S3 partition timezone: unknown time zone non-existing timezone",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			sm, err := newUploadManager(
				t.Context(),
				tc.conf,
				"metrics",
				"otlp",
				false,
			)

			if tc.errVal != "" {
				assert.Nil(t, sm, "Must not have a valid s3 upload manager")
				assert.EqualError(t, err, tc.errVal, "Must match the expected error")
			} else {
				assert.NotNil(t, sm, "Must have a valid manager")
				assert.NoError(t, err, "Must not error when creating client")
			}
		})
	}
}
