// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
	ValueHolder() interface{}
}

type valueHolderFunction func() interface{}
