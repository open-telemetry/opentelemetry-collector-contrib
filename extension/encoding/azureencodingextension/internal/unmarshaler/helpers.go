// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package unmarshaler // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/azureencodingextension/internal/unmarshaler"

type RecordsBatchFormat string

const (
	FormatEventHub    RecordsBatchFormat = "eventhub"
	FormatBlobStorage RecordsBatchFormat = "blobstorage"
)
