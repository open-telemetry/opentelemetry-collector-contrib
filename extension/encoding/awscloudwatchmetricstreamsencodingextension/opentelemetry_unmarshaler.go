// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awscloudwatchmetricstreamsencodingextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awscloudwatchmetricstreamsencodingextension"

import (
	"encoding/binary"
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awscloudwatchmetricstreamsencodingextension/internal/metadata"
)

var errUvarintReadFailure = errors.New("failed to get uvarint from record")

type formatOpenTelemetry10Unmarshaler struct {
	buildInfo component.BuildInfo
}

var _ pmetric.Unmarshaler = (*formatOpenTelemetry10Unmarshaler)(nil)

func (f formatOpenTelemetry10Unmarshaler) UnmarshalMetrics(record []byte) (pmetric.Metrics, error) {
	// if bytesRead is <= 0, there was an error
	_, bytesRead := binary.Uvarint(record)
	if bytesRead <= 0 {
		return pmetric.Metrics{}, fmt.Errorf("failed to unmarshal metrics as '%s' format: %w", formatOpenTelemetry10, errUvarintReadFailure)
	}

	// remove length prefix and get
	// only the protobuf message
	u := &pmetric.ProtoUnmarshaler{}
	metrics, err := u.UnmarshalMetrics(record[bytesRead:])
	if err != nil {
		return pmetric.Metrics{}, fmt.Errorf("failed to unmarshal metrics as '%s' format: %w", formatOpenTelemetry10, err)
	}

	// add scope name and build info version to
	// the resource metrics
	for i := 0; i < metrics.ResourceMetrics().Len(); i++ {
		rm := metrics.ResourceMetrics().At(i)
		for j := 0; j < rm.ScopeMetrics().Len(); j++ {
			sm := rm.ScopeMetrics().At(j)
			sm.Scope().SetName(metadata.ScopeName)
			sm.Scope().SetVersion(f.buildInfo.Version)
		}
	}
	return metrics, nil
}
