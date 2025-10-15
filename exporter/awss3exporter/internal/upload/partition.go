// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package upload // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awss3exporter/internal/upload"

import (
	"math/rand/v2"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/itchyny/timefmt-go"
	"go.opentelemetry.io/collector/config/configcompression"
)

var compressionFileExtensions = map[configcompression.Type]string{
	configcompression.TypeGzip: ".gz",
}

type PartitionKeyBuilder struct {
	// PartitionBasePrefix defines the root S3
	// directory (key) prefix used to write the file.
	PartitionBasePrefix string
	// PartitionPrefix defines the S3 directory (key)
	// prefix used to write the file.
	// Appended to PartitionBasePrefix if provided.
	PartitionPrefix string
	// PartitionFormat is used to separate values into
	// different time buckets.
	// Uses [strftime](https://www.man7.org/linux/man-pages/man3/strftime.3.html) formatting.
	PartitionFormat string
	// PartitionTimeLocation is used to provide timezone for partition time. Defaults to Local time location.
	PartitionTimeLocation *time.Location
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
	// body before upload.
	Compression configcompression.Type
	// UniqueKeyFunc allows for overwriting the default behavior of
	// generating a new unique string to avoid collisions on file upload
	// across many different instances.
	UniqueKeyFunc func() string
	// IsCompressed when true keeps files compressed in S3
	// by omitting ContentEncoding headers. When false, ContentEncoding
	// is set for HTTP transfer compression (AWS auto-decompresses).
	IsCompressed bool
}

func (pki *PartitionKeyBuilder) Build(ts time.Time, overridePrefix string) string {
	return path.Join(pki.bucketKeyPrefix(ts, overridePrefix), pki.fileName())
}

func (pki *PartitionKeyBuilder) bucketKeyPrefix(ts time.Time, overridePrefix string) string {
	// Don't want to overwrite the actual value
	prefix := pki.PartitionPrefix
	// Only override when it's not empty string
	if overridePrefix != "" {
		prefix = overridePrefix
	}

	var pathParts []string

	if pki.PartitionBasePrefix != "" {
		pathParts = append(pathParts, pki.PartitionBasePrefix)
	}

	if prefix != "" {
		pathParts = append(pathParts, prefix)
	}

	location := pki.PartitionTimeLocation
	if location == nil {
		location = time.Local
	}
	pathParts = append(pathParts, timefmt.Format(ts.In(location), pki.PartitionFormat))

	return strings.Join(pathParts, "/")
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
	// If a custom function is provided, use it to generate the unique key.
	// If it fails, fall back to the default random integer generation
	// so that uploads are not blocked.
	if pki.UniqueKeyFunc != nil {
		if k := pki.UniqueKeyFunc(); k != "" {
			return k
		}
	}

	return pki.randInt()
}

func GenerateUUIDv7() string {
	id, err := uuid.NewV7()
	if err != nil {
		return ""
	}
	return id.String()
}

func (*PartitionKeyBuilder) randInt() string {
	// This follows the original "uniqueness" algorithm
	// to avoid collisions on file uploads across different nodes.
	const (
		uniqueValues = 999999999
		minOffset    = 100000000
	)

	return strconv.Itoa(minOffset + rand.IntN(uniqueValues-minOffset))
}
