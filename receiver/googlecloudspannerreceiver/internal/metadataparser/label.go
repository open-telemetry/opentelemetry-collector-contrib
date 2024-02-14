// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metadataparser // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudspannerreceiver/internal/metadataparser"

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudspannerreceiver/internal/metadata"
)

type Label struct {
	Name       string             `yaml:"name"`
	ColumnName string             `yaml:"column_name"`
	ValueType  metadata.ValueType `yaml:"value_type"`
}

func (label Label) toLabelValueMetadata() (metadata.LabelValueMetadata, error) {
	return metadata.NewLabelValueMetadata(label.Name, label.ColumnName, label.ValueType)
}
