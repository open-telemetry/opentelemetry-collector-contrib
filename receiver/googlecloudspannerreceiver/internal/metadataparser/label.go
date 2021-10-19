// Copyright  The OpenTelemetry Authors
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

package metadataparser

import (
	"fmt"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudspannerreceiver/internal/metadata"
)

const (
	labelValueTypeString      = "string"
	labelValueTypeInt         = "int"
	labelValueTypeBool        = "bool"
	labelValueTypeStringSlice = "string_slice"
	labelValueTypeByteSlice   = "byte_slice"
)

type Label struct {
	Name       string `yaml:"name"`
	ColumnName string `yaml:"column_name"`
	ValueType  string `yaml:"value_type"`
}

func (label Label) toLabelValueMetadata() (metadata.LabelValueMetadata, error) {
	var valueMetadata metadata.LabelValueMetadata

	switch label.ValueType {
	case labelValueTypeString:
		valueMetadata = metadata.NewStringLabelValueMetadata(label.Name, label.ColumnName)
	case labelValueTypeInt:
		valueMetadata = metadata.NewInt64LabelValueMetadata(label.Name, label.ColumnName)
	case labelValueTypeBool:
		valueMetadata = metadata.NewBoolLabelValueMetadata(label.Name, label.ColumnName)
	case labelValueTypeStringSlice:
		valueMetadata = metadata.NewStringSliceLabelValueMetadata(label.Name, label.ColumnName)
	case labelValueTypeByteSlice:
		valueMetadata = metadata.NewByteSliceLabelValueMetadata(label.Name, label.ColumnName)
	default:
		return nil, fmt.Errorf("invalid value type received for label %q", label.Name)
	}

	return valueMetadata, nil
}
