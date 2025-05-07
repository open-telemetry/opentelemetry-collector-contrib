// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package upload // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awss3exporter/internal/upload"

import (
	"math/rand/v2"
	"strconv"
	"time"

	"github.com/itchyny/timefmt-go"
	"go.opentelemetry.io/collector/config/configcompression"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awss3exporter/internal/subst"
)

var compressionFileExtensions = map[configcompression.Type]string{
	configcompression.TypeGzip: ".gz",
}

type PartitionKeyBuilder struct {
	// PartitionPrefix defines the S3 directory (key)
	// prefix used to write the file
	// Values in bracket shell-substitution format are replaced
	// with the specified resource attributes.
	PartitionPrefix string
	// PartitionFormat is used to separate values into
	// different time buckets.
	// Uses [strftime](https://www.man7.org/linux/man-pages/man3/strftime.3.html) formatting.
	// Values in bracket shell-substitution format are replaced
	// with the specified resource attributes.
	PartitionFormat string
	// UnknownAttributePlaceholder is the value used for missing attributes in PartitionFormat.
	// If unset "_unknown_" is used.
	UnknownAttributePlaceholder string
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
	//
	// TODO: Expose the ability to config additional UniqueKeyField via config
	UniqueKeyFunc func() string
}

func (pki *PartitionKeyBuilder) Build(ts time.Time, attrs pcommon.Map) string {
	return pki.bucketKeyPrefix(ts, attrs) + "/" + pki.fileName(attrs)
}

func (pki *PartitionKeyBuilder) bucketKeyPrefix(ts time.Time, attrs pcommon.Map) string {
	// Don't want to overwrite the actual value
	prefix := pki.substAttrs(pki.PartitionPrefix, attrs)
	if prefix != "" {
		prefix += "/"
	}
	return prefix + pki.substAttrs(timefmt.Format(ts, pki.PartitionFormat), attrs)
}

func (pki *PartitionKeyBuilder) fileName(attrs pcommon.Map) string {
	var suffix string

	if pki.FileFormat != "" {
		suffix = "." + pki.FileFormat
	}

	if ext, ok := compressionFileExtensions[pki.Compression]; ok {
		suffix += ext
	}

	return pki.substAttrs(pki.FilePrefix, attrs) + pki.Metadata + "_" + pki.uniqueKey() + suffix
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

func (pki *PartitionKeyBuilder) substAttrs(s string, attrs pcommon.Map) string {
	unknown := "_unknown_"
	if ph := pki.UnknownAttributePlaceholder; ph != "" {
		unknown = ph
	}
	return subst.Subst(s, func(key string) string {
		val, ok := attrs.Get(key)
		if !ok {
			return unknown
		}
		str := val.AsString()
		if str == "" {
			return unknown
		}
		return str
	})
}
