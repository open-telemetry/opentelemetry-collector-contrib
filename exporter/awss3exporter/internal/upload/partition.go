// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package upload // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awss3exporter/internal/upload"

import (
	"fmt"
	"math/rand/v2"
	"strconv"
	"time"

	"go.opentelemetry.io/collector/config/configcompression"
)

var compressionFileExtensions = map[configcompression.Type]string{
	configcompression.TypeGzip: ".gz",
}

type PartitionKeyBuilder struct {
	// PartitionPrefix defines the S3 directory (key)
	// prefix used to write the file
	PartitionPrefix string
	// PartitionTruncation is used to truncate values into
	// different time buckets.
	// Currently hourly or minutely is supported
	PartitionTruncation string
	// FilePrefix is used to define the prefix of the file written
	// to the directory in S3.
	FilePrefix string
	// FileFormat defines what encoding was used to write
	// the content to s3
	FileFormat string
	// Metadata provides additional details regarding the file
	// Expected to be one of "metrics", "traces", or "logs"
	Metadata string
	// Compression defines algorithm used on the
	// body before uploaded.
	Compression configcompression.Type
	// UniqueKeyFunc allows for overwritting the default behavior of
	// generating a new unique string to avoid collisions on file upload
	// across many different instances.
	//
	// TODO: Expose the ability to config additional UniqueKeyField via config
	UniqueKeyFunc func() string
}

func (pki *PartitionKeyBuilder) Build(ts time.Time) string {
	return pki.bucketKeyPrefix(ts) + "/" + pki.fileName()
}

func (pki *PartitionKeyBuilder) bucketKeyPrefix(ts time.Time) string {
	key := fmt.Sprintf("year=%d/month=%02d/day=%02d/hour=%02d", ts.Year(), ts.Month(), ts.Day(), ts.Hour())

	switch pki.PartitionTruncation {
	case "minute":
		key += "/" + fmt.Sprintf("minute=%02d", ts.Minute())
	default:
		// Nothing to do, key defaults to hourly
	}

	return pki.PartitionPrefix + "/" + key
}

func (pki *PartitionKeyBuilder) fileName() string {
	var suffix string

	if pki.FileFormat != "" {
		suffix = "." + pki.FileFormat
	}

	if ext, ok := compressionFileExtensions[pki.Compression]; ok {
		suffix += ext
	}

	return pki.FilePrefix + pki.Metadata + "_" + pki.uniqueKey() + suffix
}

func (pki *PartitionKeyBuilder) uniqueKey() string {
	if pki.UniqueKeyFunc != nil {
		return pki.UniqueKeyFunc()
	}

	// This follows the original "uniqueness" algorithm
	// to avoid collisions on file uploads across different nodes.
	const (
		uniqueValues = 999999999
		minOffset    = 100000000
	)

	return strconv.Itoa(minOffset + rand.IntN(uniqueValues-minOffset))
}
