// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metadata // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudspannerreceiver/internal/metadata"

type ValueType string

const (
	UnknownValueType          ValueType = "unknown"
	StringValueType           ValueType = "string"
	IntValueType              ValueType = "int"
	FloatValueType            ValueType = "float"
	NullFloatValueType        ValueType = "null_float"
	BoolValueType             ValueType = "bool"
	StringSliceValueType      ValueType = "string_slice"
	ByteSliceValueType        ValueType = "byte_slice"
	LockRequestSliceValueType ValueType = "lock_request_slice"
)

type ValueMetadata interface {
	Name() string
	ColumnName() string
	ValueHolder() any
}

type valueHolderFunction func() any
